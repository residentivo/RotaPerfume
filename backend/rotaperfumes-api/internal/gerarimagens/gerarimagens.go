// Package gerarimagens é o comando cmd/gerarimagens (IMG-01, D4) fora do
// main: gera um PNG por produto com o nome (descrição) escrito na diagonal e
// o grava pelo MESMO caso de uso do upload (catalogoapp.DefinirImagemProduto:
// mesma validação de conteúdo, mesmo nome UUID v4, mesma troca transacional),
// sobre o mesmo PRODUTO_IMAGENS_DIR da API. Nunca grava direto no disco nem
// no banco.
//
// Exige -confirmar para gravar; -dry-run só lista (e renderiza) sem gravar.
// Produtos que já têm imagem são pulados, salvo -sobrescrever.
package gerarimagens

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

// Tag prefixa as mensagens de erro do comando.
const Tag = "gerarimagens"

const (
	prazoComando   = 30 * time.Minute
	itensPorPagina = 100
)

// ErrSemConfirmacao: o comando grava em disco e no banco; sem -confirmar
// (e sem -dry-run) nada é feito.
var ErrSemConfirmacao = errors.New("nada foi feito: use -confirmar para gravar (ou -dry-run para simular)")

// Catalogo são os casos de uso que o comando usa (catalogoapp.Casos
// satisfaz).
type Catalogo interface {
	ListarProdutos(ctx context.Context, f catalogo.FiltroProduto, p kernel.Pagina) (kernel.Resultado[catalogo.Produto], error)
	DefinirImagemProduto(ctx context.Context, id int64, dados []byte) (*catalogo.Produto, error)
}

// Opcoes são as flags do comando.
type Opcoes struct {
	Sobrescrever bool
	DryRun       bool
}

// Totais resumem a execução.
type Totais struct {
	Lidos, Gerados, Pulados, Simulados, Falhas int
}

// Deps são as dependências do comando que não vêm do cmdutil.Ambiente.
type Deps struct {
	// AbrirBanco abre o pool (db.Open: ping de 5 s e DSN mascarado).
	AbrirBanco func(dsn string) (*sql.DB, error)
	// MontarCatalogo liga os casos de uso; gravar=false (dry-run) monta sem
	// armazenamento de imagens (nada é criado no disco).
	MontarCatalogo func(cfg *config.Config, pool *sql.DB, gravar bool) (Catalogo, error)
}

// DepsPadrao liga o MariaDB, o adapter de filesystem e os casos de uso
// reais pela composition root.
func DepsPadrao() Deps {
	return Deps{AbrirBanco: db.Open, MontarCatalogo: montarCatalogo}
}

func montarCatalogo(cfg *config.Config, pool *sql.DB, gravar bool) (Catalogo, error) {
	if !gravar {
		return bootstrap.NovoCatalogo(pool, nil), nil
	}
	arm, err := bootstrap.AbrirArmazenamentoImagens(cfg.ProdutoImagensDir)
	if err != nil {
		return nil, err
	}
	return bootstrap.NovoCatalogo(pool, arm), nil
}

// CLI devolve o comando; o main só chama cmdutil.Executar.
func CLI(deps Deps) cmdutil.Comando {
	return func(args []string, amb cmdutil.Ambiente) error {
		fs := cmdutil.NovasFlags(Tag, amb)
		confirmar := fs.Bool("confirmar", false, "grava as imagens geradas (obrigatório para gravar)")
		sobrescrever := fs.Bool("sobrescrever", false, "gera também para produtos que já têm imagem (troca a imagem)")
		dryRun := fs.Bool("dry-run", false, "só lista o que seria gerado, sem gravar nada")
		if err := cmdutil.LerFlags(fs, args); err != nil {
			return err
		}
		if !*confirmar && !*dryRun {
			fmt.Fprintln(amb.Erro, ErrSemConfirmacao.Error())
			return &cmdutil.ErroDeUso{Err: ErrSemConfirmacao}
		}
		cfg, err := cmdutil.Configurar(amb, false)
		if err != nil {
			return err
		}
		return rodar(cfg, Opcoes{Sobrescrever: *sobrescrever, DryRun: *dryRun}, amb, deps)
	}
}

func rodar(cfg *config.Config, op Opcoes, amb cmdutil.Ambiente, deps Deps) error {
	pool, err := deps.AbrirBanco(cfg.DSN())
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), prazoComando)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}
	cat, err := deps.MontarCatalogo(cfg, pool, !op.DryRun)
	if err != nil {
		return fmt.Errorf("imagens de produto: %w", err)
	}
	fmt.Fprintf(amb.Saida, "Banco: %s@%s:%s | diretório: %s | modo: %s\n",
		cfg.DBName, cfg.DBHost, cfg.DBPort, cfg.ProdutoImagensDir, modo(op))
	t, err := Gerar(ctx, cat, op, amb.Saida)
	ImprimirTotais(amb.Saida, t)
	return err
}

func modo(op Opcoes) string {
	switch {
	case op.DryRun:
		return "dry-run"
	case op.Sobrescrever:
		return "gravar (sobrescrever)"
	default:
		return "gravar (pula quem já tem imagem)"
	}
}

// Gerar percorre todos os produtos (páginas de 100, por id) e gera a imagem
// de cada um conforme as opções. Uma falha num produto não interrompe o
// lote; no fim, Falhas > 0 vira erro.
func Gerar(ctx context.Context, cat Catalogo, op Opcoes, out io.Writer) (Totais, error) {
	var t Totais
	filtro := catalogo.FiltroProduto{Ordenacao: catalogo.OrdenacaoProdutoPadrao()}
	for pagina := 1; ; pagina++ {
		res, err := cat.ListarProdutos(ctx, filtro, kernel.NovaPagina(pagina, itensPorPagina))
		if err != nil {
			return t, fmt.Errorf("listar produtos: %w", err)
		}
		for _, p := range res.Itens {
			t.Lidos++
			processar(ctx, cat, op, p, &t, out)
		}
		if pagina >= res.Paginas() || len(res.Itens) == 0 {
			break
		}
	}
	vlog.Printf("gerarimagens.go", "Gerar", "totais lidos=%d gerados=%d pulados=%d simulados=%d falhas=%d",
		t.Lidos, t.Gerados, t.Pulados, t.Simulados, t.Falhas)
	if t.Falhas > 0 {
		return t, fmt.Errorf("%d produto(s) falharam", t.Falhas)
	}
	return t, nil
}

func processar(ctx context.Context, cat Catalogo, op Opcoes, p catalogo.Produto, t *Totais, out io.Writer) {
	if p.Imagem != "" && !op.Sobrescrever {
		t.Pulados++
		return
	}
	png, err := RenderizarPNG(p.Descricao, p.SKU.String())
	if err != nil {
		t.Falhas++
		fmt.Fprintf(out, "FALHA  id=%d sku=%s: renderizar: %v\n", p.ID, p.SKU.String(), err)
		return
	}
	if op.DryRun {
		t.Simulados++
		fmt.Fprintf(out, "SIMULA id=%d sku=%s bytes=%d\n", p.ID, p.SKU.String(), len(png))
		return
	}
	novo, err := cat.DefinirImagemProduto(ctx, p.ID, png)
	if err != nil {
		t.Falhas++
		fmt.Fprintf(out, "FALHA  id=%d sku=%s: %v\n", p.ID, p.SKU.String(), err)
		return
	}
	t.Gerados++
	fmt.Fprintf(out, "OK     id=%d sku=%s arquivo=%s\n", p.ID, p.SKU.String(), novo.Imagem)
}

// ImprimirTotais escreve o resumo da execução.
func ImprimirTotais(out io.Writer, t Totais) {
	fmt.Fprintf(out, "Total: %d | gerados: %d | pulados (já tinham imagem): %d | simulados: %d | falhas: %d\n",
		t.Lidos, t.Gerados, t.Pulados, t.Simulados, t.Falhas)
}
