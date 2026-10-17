package arquitetura_test

// ARQ-05 (E-02/O1): todo método Listar* dos adapters de vendasmysql recebe
// kernel.Escopo (nunca vendedorID int64 de escopo); com controle negativo.

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const pkgVendasMySQL = modShared + "/vendas/vendasmysql"

func violacoesListarVendasSemEscopo(arqs []arquivo) (violacoes []string, listar int) {
	for _, a := range arqs {
		if a.pkg != pkgVendasMySQL {
			continue
		}
		alias := nomesDoImport(a.f, pkgKernel)
		for _, d := range a.f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !strings.HasPrefix(fn.Name.Name, "Listar") {
				continue
			}
			listar++
			if !recebeEscopo(fn, alias) {
				violacoes = append(violacoes, a.pos(fn.Pos())+": "+fn.Name.Name+" sem parâmetro kernel.Escopo")
			}
		}
	}
	return violacoes, listar
}

func TestARQ05_ListagensVendasRecebemEscopo(t *testing.T) {
	violacoes, listar := violacoesListarVendasSemEscopo(todosOsArquivos(t))
	assert.Empty(t, violacoes)
	assert.GreaterOrEqual(t, listar, 2, "Pedidos.Listar e Pagamentos.Listar")
}

func TestARQ05_ListagensVendas_DetectaViolacao(t *testing.T) {
	adapter := fonteSintetica(t, pkgVendasMySQL, "pedidos.go", `package vendasmysql
import "github.com/rotaperfumes/shared/kernel"
type R struct{}
func (R) Listar(ctx any, vendedorID int64) {}
func (R) ListarOutro(ctx any, e kernel.Escopo) {}
func (R) PorID(ctx any, id int64) {}
`)
	violacoes, listar := violacoesListarVendasSemEscopo([]arquivo{adapter})
	assert.Len(t, violacoes, 1)
	assert.Equal(t, 2, listar)
}
