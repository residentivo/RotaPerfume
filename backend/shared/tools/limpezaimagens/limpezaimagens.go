// Package limpezaimagens implementa o comando cmd/limpezaimagens (CHORE-09):
// apaga do PRODUTO_IMAGENS_DIR as imagens de produto que nenhum
// produtos.imagem referencia e os temporários .tmp-* abandonados.
//
// Uso (a partir de backend/shared, com o .env da raiz):
//
//	go run ./cmd/limpezaimagens -dry-run   # só relata o que apagaria
//	go run ./cmd/limpezaimagens            # apaga
//
// Regras (🟣 SecBrain, Lote 31):
//   - todo acesso ao diretório passa por um *os.Root (nenhum nome escapa do
//     diretório); symlinks e o que não for arquivo regular são ignorados,
//     nunca seguidos;
//   - só nomes <uuid-v4>.(jpg|png|webp) (catalogo.NomeImagemValido) sem
//     referência no banco são apagados, e só com idade mínima de 1 h (uma
//     transação de troca de imagem ainda aberta não perde o arquivo);
//   - .tmp-* (inclusive .tmp-probe-*) só com idade mínima de 24 h;
//   - qualquer outro nome é ignorado;
//   - banco apontando para arquivo inexistente: só reporta (contagem);
//   - roda com a conta da API (só SELECT em produtos);
//   - o log tem só totais: nenhum nome de arquivo nem caminho absoluto.
package limpezaimagens

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/cmdutil"
)

// Tag prefixa os logs e as mensagens de erro do comando.
const Tag = "limpezaimagens"

// Idades mínimas para apagar (CHORE-09).
const (
	IdadeMinimaOrfao = time.Hour
	IdadeMinimaTemp  = 24 * time.Hour
	prefixoTemp      = ".tmp-"
	sqlReferencias   = `SELECT imagem FROM produtos WHERE imagem IS NOT NULL AND imagem <> ''`
)

// Options configura uma execução.
type Options struct {
	Dir    string
	DryRun bool
	// Agora é o relógio (nil = time.Now).
	Agora func() time.Time
}

// Relatorio são os totais da execução (o único conteúdo logado).
type Relatorio struct {
	Referenciadas         int // arquivos com referência no banco (preservados)
	OrfasApagadas         int // órfãs apagadas (ou que seriam, no dry-run)
	OrfasRecentes         int // órfãs mais novas que IdadeMinimaOrfao (preservadas)
	TempsApagados         int // .tmp-* apagados (ou que seriam)
	TempsRecentes         int // .tmp-* mais novos que IdadeMinimaTemp
	Ignorados             int // nome fora do padrão, symlink, diretório etc.
	ReferenciasSemArquivo int // produtos.imagem apontando para arquivo inexistente (só reporta)
	FalhasAoApagar        int
}

// Run abre o banco (open), lê as referências e limpa o diretório.
func Run(opts Options, open cmdutil.Opener) (Relatorio, error) {
	conn, err := open()
	if err != nil {
		return Relatorio{}, fmt.Errorf("abrir banco: %w", err)
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		return Relatorio{}, fmt.Errorf("conectar ao banco: %w", err)
	}
	refs, err := referencias(conn)
	if err != nil {
		return Relatorio{}, err
	}
	return Limpar(opts, refs)
}

// referencias lê os nomes gravados em produtos.imagem.
func referencias(db cmdutil.DB) (map[string]bool, error) {
	rows, err := db.Query(sqlReferencias)
	if err != nil {
		return nil, fmt.Errorf("ler produtos.imagem: %w", err)
	}
	defer rows.Close()
	refs := map[string]bool{}
	for rows.Next() {
		var nome string
		if err := rows.Scan(&nome); err != nil {
			return nil, fmt.Errorf("ler produtos.imagem: %w", err)
		}
		refs[nome] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ler produtos.imagem: %w", err)
	}
	return refs, nil
}

// Limpar aplica as regras ao diretório com as referências já lidas.
func Limpar(opts Options, refs map[string]bool) (Relatorio, error) {
	agora := time.Now
	if opts.Agora != nil {
		agora = opts.Agora
	}
	root, err := os.OpenRoot(opts.Dir)
	if err != nil {
		return Relatorio{}, fmt.Errorf("abrir PRODUTO_IMAGENS_DIR: %w", err)
	}
	defer root.Close()
	entradas, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return Relatorio{}, fmt.Errorf("listar PRODUTO_IMAGENS_DIR: %w", err)
	}
	var r Relatorio
	presentes := map[string]bool{}
	for _, e := range entradas {
		presentes[e.Name()] = true
		acao := classificar(e, refs, agora(), &r)
		if acao && !opts.DryRun {
			if err := root.Remove(e.Name()); err != nil && !errors.Is(err, fs.ErrNotExist) {
				r.FalhasAoApagar++
			}
		}
	}
	for nome := range refs {
		if !presentes[nome] {
			r.ReferenciasSemArquivo++
		}
	}
	logar(r, opts.DryRun)
	return r, nil
}

// classificar conta a entrada e diz se ela deve ser apagada. Só arquivo
// regular (o DirEntry do os não segue symlink) entra nas regras.
func classificar(e fs.DirEntry, refs map[string]bool, agora time.Time, r *Relatorio) bool {
	if !e.Type().IsRegular() {
		r.Ignorados++
		return false
	}
	info, err := e.Info()
	if err != nil || !info.Mode().IsRegular() {
		r.Ignorados++
		return false
	}
	idade := agora.Sub(info.ModTime())
	nome := e.Name()
	switch {
	case catalogo.NomeImagemValido(nome) && refs[nome]:
		r.Referenciadas++
	case catalogo.NomeImagemValido(nome) && idade >= IdadeMinimaOrfao:
		r.OrfasApagadas++
		return true
	case catalogo.NomeImagemValido(nome):
		r.OrfasRecentes++
	case strings.HasPrefix(nome, prefixoTemp) && idade >= IdadeMinimaTemp:
		r.TempsApagados++
		return true
	case strings.HasPrefix(nome, prefixoTemp):
		r.TempsRecentes++
	default:
		r.Ignorados++
	}
	return false
}

// logar escreve só os totais (nunca nomes nem caminhos).
func logar(r Relatorio, dryRun bool) {
	log.Printf("%s: dry_run=%t referenciadas=%d orfas_apagadas=%d orfas_recentes=%d temps_apagados=%d temps_recentes=%d ignorados=%d referencias_sem_arquivo=%d falhas_ao_apagar=%d",
		Tag, dryRun, r.Referenciadas, r.OrfasApagadas, r.OrfasRecentes, r.TempsApagados, r.TempsRecentes, r.Ignorados, r.ReferenciasSemArquivo, r.FalhasAoApagar)
}
