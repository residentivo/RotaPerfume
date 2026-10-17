package arquitetura_test

// ARQ-04 (§7.6 aceite, O5, E-02): duas regras a mais sobre o escopo de
// carteira, nos dois módulos (_test.go fora):
//   - os construtores kernel.Escopo* só aparecem no ARQUIVO do
//     AutorizadorEscopo (shared/comercial/comercialapp/escopo.go), mais
//     estreito que TestARQ_Escopo_SoAutorizador (que libera o pacote todo);
//   - todo método Listar* dos adapters comercialmysql recebe kernel.Escopo
//     (nenhuma listagem com vendedorID int64 de escopo).

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const arquivoAutorizador = "comercial/comercialapp/escopo.go"

func violacoesEscopoForaDoAutorizador(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		if a.pkg == pkgKernel || strings.HasSuffix(a.caminho, arquivoAutorizador) {
			continue
		}
		referenciasA(a, pkgKernel, func(nome string, sel *ast.SelectorExpr) {
			switch nome {
			case "EscopoIrrestrito", "EscopoDoVendedor", "EscopoSemAcesso":
				out = append(out, a.pos(sel.Pos())+": "+nome+" fora de "+arquivoAutorizador)
			}
		})
	}
	return out
}

func violacoesListarSemEscopo(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		if a.pkg != modShared+"/comercial/comercialmysql" {
			continue
		}
		alias := nomesDoImport(a.f, pkgKernel)
		for _, d := range a.f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !strings.HasPrefix(fn.Name.Name, "Listar") {
				continue
			}
			if !recebeEscopo(fn, alias) {
				out = append(out, a.pos(fn.Pos())+": "+fn.Name.Name+" sem parâmetro kernel.Escopo")
			}
		}
	}
	return out
}

func recebeEscopo(fn *ast.FuncDecl, alias string) bool {
	for _, campo := range fn.Type.Params.List {
		if sel, ok := campo.Type.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == alias && sel.Sel.Name == "Escopo" {
				return true
			}
		}
	}
	return false
}

func TestARQ04_Escopo_SoNoArquivoDoAutorizador(t *testing.T) {
	arqs := todosOsArquivos(t)
	assert.Empty(t, violacoesEscopoForaDoAutorizador(arqs))

	vistos := 0
	for _, a := range arqs {
		if strings.HasSuffix(a.caminho, arquivoAutorizador) {
			referenciasA(a, pkgKernel, func(nome string, _ *ast.SelectorExpr) {
				if strings.HasPrefix(nome, "Escopo") {
					vistos++
				}
			})
		}
	}
	assert.Positive(t, vistos, "a varredura deveria ver os construtores no autorizador")
}

func TestARQ04_ListagensComercialRecebemEscopo(t *testing.T) {
	arqs := todosOsArquivos(t)
	assert.Empty(t, violacoesListarSemEscopo(arqs))

	listar := 0
	for _, a := range arqs {
		if a.pkg == modShared+"/comercial/comercialmysql" {
			for _, d := range a.f.Decls {
				if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv != nil && strings.HasPrefix(fn.Name.Name, "Listar") {
					listar++
				}
			}
		}
	}
	assert.GreaterOrEqual(t, listar, 4, "Vendedores.ListarResumo e os Listar de clientes, visitas e oportunidades")
}

func TestARQ04_RegrasDeEscopo_DetectamViolacoes(t *testing.T) {
	const imp = `import "github.com/rotaperfumes/shared/kernel"`
	ruim := fonteSintetica(t, modShared+"/comercial/comercialapp", "comercial/comercialapp/clientes.go", "package comercialapp\n"+imp+`
func f() { _ = kernel.EscopoSemAcesso(); g := kernel.EscopoDoVendedor; _ = g }
`)
	assert.Len(t, violacoesEscopoForaDoAutorizador([]arquivo{ruim}), 2)
	bom := fonteSintetica(t, modShared+"/comercial/comercialapp", arquivoAutorizador, "package comercialapp\n"+imp+`
func f() { _ = kernel.EscopoSemAcesso() }
`)
	assert.Empty(t, violacoesEscopoForaDoAutorizador([]arquivo{bom}))

	adapter := fonteSintetica(t, modShared+"/comercial/comercialmysql", "visitas.go", "package comercialmysql\n"+imp+`
type R struct{}
func (R) Listar(ctx any, vendedorID int64) {}
func (R) ListarResumo(ctx any, e kernel.Escopo) {}
func (R) PorID(ctx any, id int64) {}
`)
	assert.Len(t, violacoesListarSemEscopo([]arquivo{adapter}), 1)
}
