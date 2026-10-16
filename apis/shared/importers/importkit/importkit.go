// Package importkit é o esqueleto comum dos importadores CSV (ARQ-07,
// Template Method, docs/arquitetura-ddd.md §5.13). O pipeline é sempre o
// mesmo: ler → validar → transformar (sem banco) → [dry-run para aqui] →
// abrir e pingar o banco → gravar em lote (lookups + upsert) → relatório.
// Cada importador só fornece os passos variáveis (Etapas) e o SQL de upsert,
// que continua no próprio importador (carga em lote não passa pelos
// repositórios de agregado). Valores sempre por placeholder; CNPJ nunca vai
// para o log (M-04); nenhum vlog por linha (F-09).
package importkit

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/vlog"
)

const arq = "importkit.go"

// Etapas são os passos variáveis de um importador (os ganchos do Template
// Method). Ler e Gravar são obrigatórios; LerNoDryRun é opcional.
type Etapas struct {
	// Tag prefixa os logs ("importprodutos").
	Tag string
	// Ler resolve e lê o(s) CSV(s), valida e transforma as linhas, e loga as
	// contagens. Não toca o banco.
	Ler func() error
	// LerNoDryRun roda só no dry-run, antes do aviso final (ex.: o
	// importpedidos lê o CSV de itens, que na carga real é lido depois da
	// fase 1).
	LerNoDryRun func() error
	// Gravar carrega os lookups, grava em lote e loga o relatório.
	Gravar func(db cmdutil.DB) error
}

// Executar é o método-template: a ordem dos passos é fixa para todos os
// importadores (a mesma dos comandos originais).
func Executar(e Etapas, dryRun bool, open cmdutil.Opener) (err error) {
	defer vlog.Trace(arq, "Executar", "tag=%s dry_run=%t", e.Tag, dryRun)(&err)
	if err := e.Ler(); err != nil {
		return err
	}
	if dryRun {
		if e.LerNoDryRun != nil {
			if err := e.LerNoDryRun(); err != nil {
				return err
			}
		}
		log.Printf("%s: --dry-run informado, nada foi gravado no banco", e.Tag)
		return nil
	}
	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping no banco falhou: %w", err)
	}
	return e.Gravar(db)
}

// ResolverCSV decide o caminho do CSV, na ordem: flag > variável de ambiente
// > padrão (partes relativas à raiz do repositório, ex.: "dados", "erp",
// "produtos.csv").
func ResolverCSV(flag, envVar string, padrao ...string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	raiz, err := cmdutil.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a raiz do projeto: %w", err)
	}
	return filepath.Join(append([]string{raiz}, padrao...)...), nil
}

// Formato descreve um CSV com cabeçalho: o prefixo dos logs por linha com
// erro, antes de " linha N" ("importvisitas:" ou "importpedidos: pedidos.csv"),
// e o número exato de campos por registro.
type Formato struct {
	Prefixo string
	Campos  int
}

// LerArquivo abre o arquivo e delega a leitura.
func LerArquivo[R any](caminho string, ler func(io.Reader) ([]R, int, error)) ([]R, int, error) {
	f, err := os.Open(caminho)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()
	return ler(f)
}

// LerCSV descarta o cabeçalho e converte cada registro com parse. Linhas
// malformadas são contadas em invalidas, logadas pelo número da linha e
// puladas: não abortam a importação.
func LerCSV[R any](rd io.Reader, f Formato, parse func([]string) (R, error)) (linhas []R, invalidas int, err error) {
	r := csv.NewReader(rd)
	r.FieldsPerRecord = f.Campos
	if _, err := r.Read(); err != nil {
		return nil, 0, fmt.Errorf("lendo cabeçalho: %w", err)
	}
	numero := 1
	for {
		registro, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		numero++
		if err != nil {
			log.Printf("%s linha %d: erro de leitura CSV: %v", f.Prefixo, numero, err)
			invalidas++
			continue
		}
		linha, err := parse(registro)
		if err != nil {
			log.Printf("%s linha %d: %v", f.Prefixo, numero, err)
			invalidas++
			continue
		}
		linhas = append(linhas, linha)
	}
	vlog.Printf(arq, "LerCSV", "linhas_validas=%d invalidas=%d", len(linhas), invalidas)
	return linhas, invalidas, nil
}
