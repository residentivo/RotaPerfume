package arquitetura_test

// Allowlists de chamadores e testes de AST (§4.3; O1, O4, O5, O6, R-c, R-g).
// Os arquivos _test.go ficam fora (go/build só entrega GoFiles). Pacotes que
// ainda não existem são ignorados: as regras valem desde o ARQ-01.

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Regras
// ---------------------------------------------------------------------------

// referenciasA visita TODA referência qualificada `alias.Nome` ao pacote
// alvo — chamada ou não (N-4: `f := kernel.NovoPrincipal` também conta).
func referenciasA(a arquivo, alvo string, visitar func(nome string, sel *ast.SelectorExpr)) {
	alias := nomesDoImport(a.f, alvo)
	if alias == "" {
		return
	}
	ast.Inspect(a.f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == alias {
			visitar(sel.Sel.Name, sel)
		}
		return true
	})
}

// O5: kernel.NovoPrincipal só em rotaperfumes-api/internal/httpx.
func violacoesNovoPrincipal(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		referenciasA(a, pkgKernel, func(nome string, sel *ast.SelectorExpr) {
			if nome == "NovoPrincipal" && a.pkg != pkgHttpx {
				out = append(out, a.pos(sel.Pos())+": NovoPrincipal fora de internal/httpx")
			}
		})
	}
	return out
}

// O5: construtores Escopo* só em shared/comercial/comercialapp e no kernel
// (dentro do kernel as referências não são qualificadas).
func violacoesEscopo(arqs []arquivo) []string {
	permitidos := map[string]bool{modShared + "/comercial/comercialapp": true, pkgKernel: true}
	var out []string
	for _, a := range arqs {
		referenciasA(a, pkgKernel, func(nome string, sel *ast.SelectorExpr) {
			switch nome {
			case "EscopoIrrestrito", "EscopoDoVendedor", "EscopoSemAcesso":
				if !permitidos[a.pkg] {
					out = append(out, a.pos(sel.Pos())+": "+nome+" fora do AutorizadorEscopo")
				}
			}
		})
	}
	return out
}

// N-5: o middleware de autenticação legado só é montado pelo Autenticador
// (internal/httpx/auth.go); testes (_test.go) ficam fora da varredura.
func violacoesMiddlewareAuthLegado(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		permitido := a.pkg == pkgHttpx && strings.HasSuffix(a.caminho, "/auth.go")
		referenciasA(a, modAPI+"/middleware", func(nome string, sel *ast.SelectorExpr) {
			switch nome {
			case "JWTMiddleware", "JWTMiddlewareWithUserCheck", "RequireAdmin":
				if !permitido {
					out = append(out, a.pos(sel.Pos())+": middleware."+nome+" fora de internal/httpx/auth.go")
				}
			}
		})
	}
	return out
}

// O6-iii: kernel.NovoErro só em `var` de pacote, Kind constante e mensagem
// literal string.
func violacoesNovoErro(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		emVar := posicoesEmVarDePacote(a.f)
		alias := nomesDoImport(a.f, pkgKernel)
		chamadasA(a, pkgKernel, func(nome string, call *ast.CallExpr) {
			if nome != "NovoErro" {
				return
			}
			onde := a.pos(call.Pos()) + ": NovoErro "
			if !emVar(call.Pos()) {
				out = append(out, onde+"fora de declaração var de pacote")
			}
			if len(call.Args) != 2 {
				out = append(out, onde+"com aridade inesperada")
				return
			}
			if !ehConstanteKind(call.Args[0], alias, a.pkg == pkgKernel) {
				out = append(out, onde+"sem Kind constante (kernel.Kind*)")
			}
			if lit, ok := call.Args[1].(*ast.BasicLit); !ok || lit.Kind != token.STRING {
				out = append(out, onde+"sem mensagem literal string")
			}
		})
	}
	return out
}

// posicoesEmVarDePacote devolve um teste "a posição está dentro de um var de
// nível de pacote?".
func posicoesEmVarDePacote(f *ast.File) func(token.Pos) bool {
	type faixa struct{ ini, fim token.Pos }
	var faixas []faixa
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.VAR {
			faixas = append(faixas, faixa{g.Pos(), g.End()})
		}
	}
	return func(p token.Pos) bool {
		for _, fx := range faixas {
			if p >= fx.ini && p < fx.fim {
				return true
			}
		}
		return false
	}
}

func ehConstanteKind(e ast.Expr, alias string, noKernel bool) bool {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		return ok && id.Name == alias && strings.HasPrefix(x.Sel.Name, "Kind") && len(x.Sel.Name) > len("Kind")
	case *ast.Ident:
		return noKernel && strings.HasPrefix(x.Name, "Kind") && len(x.Name) > len("Kind")
	}
	return false
}

// R-c: errors.Join proibido nos pacotes de contexto (domínio, app, infra).
func violacoesErrorsJoin(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		if !pacoteDeContexto(a.pkg) {
			continue
		}
		chamadasA(a, "errors", func(nome string, call *ast.CallExpr) {
			if nome == "Join" {
				out = append(out, a.pos(call.Pos())+": errors.Join em pacote de contexto")
			}
		})
	}
	return out
}

// O4: context.WithValue com keyUserID/keyRole só no middleware legado; se
// ComPrincipalLegado vier a existir, só internal/httpx/auth.go o chama.
func violacoesChavesLegadas(arqs []arquivo) []string {
	const arquivoLegado = "middleware/auth_middleware.go"
	var out []string
	for _, a := range arqs {
		chamadasA(a, "context", func(nome string, call *ast.CallExpr) {
			if nome != "WithValue" || len(call.Args) != 3 {
				return
			}
			if id, ok := call.Args[1].(*ast.Ident); ok && (id.Name == "keyUserID" || id.Name == "keyRole") {
				if !(a.pkg == modAPI+"/middleware" && a.caminho == arquivoLegado) {
					out = append(out, a.pos(call.Pos())+": chave legada "+id.Name+" gravada fora do middleware legado")
				}
			}
		})
		chamadasA(a, modAPI+"/middleware", func(nome string, call *ast.CallExpr) {
			if nome == "ComPrincipalLegado" && !(a.pkg == pkgHttpx && strings.HasSuffix(a.caminho, "auth.go")) {
				out = append(out, a.pos(call.Pos())+": ComPrincipalLegado fora de internal/httpx/auth.go")
			}
		})
	}
	return out
}

// O1: nenhum adapter *mysql usa Escopo.VendedorID()/Irrestrito() para decidir
// SQL; só Escopo.Filtro(). Resolve, por função, os parâmetros e variáveis
// declarados como kernel.Escopo.
func violacoesFiltro(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		if !strings.HasPrefix(a.pkg, modShared+"/") || !strings.HasSuffix(a.pkg, "mysql") {
			continue
		}
		alias := nomesDoImport(a.f, pkgKernel)
		for _, d := range a.f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			escopos := nomesDoTipoEscopo(fn, alias)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "VendedorID" && sel.Sel.Name != "Irrestrito") {
					return true
				}
				if id, ok := sel.X.(*ast.Ident); ok && escopos[id.Name] {
					out = append(out, a.pos(call.Pos())+": Escopo."+sel.Sel.Name+"() no adapter (use Filtro)")
				}
				return true
			})
		}
	}
	return out
}

func nomesDoTipoEscopo(fn *ast.FuncDecl, alias string) map[string]bool {
	nomes := map[string]bool{}
	ehEscopo := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == alias && sel.Sel.Name == "Escopo"
	}
	for _, campo := range fn.Type.Params.List {
		if ehEscopo(campo.Type) {
			for _, n := range campo.Names {
				nomes[n.Name] = true
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if vs, ok := n.(*ast.ValueSpec); ok && vs.Type != nil && ehEscopo(vs.Type) {
			for _, nome := range vs.Names {
				nomes[nome.Name] = true
			}
		}
		return true
	})
	return nomes
}

// ---------------------------------------------------------------------------
// Testes sobre o código real
// ---------------------------------------------------------------------------

func todosOsArquivos(t *testing.T) []arquivo {
	t.Helper()
	var out []arquivo
	for _, p := range pacotesDosDoisModulos(t) {
		out = append(out, p.arquivos...)
	}
	return out
}

func TestARQ_NovoPrincipal_SoHttpx(t *testing.T) {
	assert.Empty(t, violacoesNovoPrincipal(todosOsArquivos(t)))
}

func TestARQ_Escopo_SoAutorizador(t *testing.T) {
	assert.Empty(t, violacoesEscopo(todosOsArquivos(t)))
}

func TestARQ_NovoErro_SentinelaLiteral(t *testing.T) {
	arqs := todosOsArquivos(t)
	assert.Empty(t, violacoesNovoErro(arqs))

	// Sanidade: a regra enxerga os sentinelas que já existem.
	total := 0
	for _, a := range arqs {
		chamadasA(a, pkgKernel, func(nome string, _ *ast.CallExpr) {
			if nome == "NovoErro" {
				total++
			}
		})
	}
	assert.GreaterOrEqual(t, total, 5, "a varredura deveria ver os sentinelas do kernel e do httpx")
}

func TestARQ_SemErrorsJoinDeDominio(t *testing.T) {
	assert.Empty(t, violacoesErrorsJoin(todosOsArquivos(t)))
}

func TestARQ_ChavesLegadas_SoMiddlewareLegado(t *testing.T) {
	arqs := todosOsArquivos(t)
	assert.Empty(t, violacoesChavesLegadas(arqs))

	// Sanidade: o único gravador legado continua sendo visto.
	achou := 0
	for _, a := range arqs {
		chamadasA(a, "context", func(nome string, call *ast.CallExpr) {
			if nome == "WithValue" && len(call.Args) == 3 {
				if id, ok := call.Args[1].(*ast.Ident); ok && strings.HasPrefix(id.Name, "key") {
					achou++
				}
			}
		})
	}
	assert.Equal(t, 2, achou, "o middleware legado grava keyUserID e keyRole")
}

func TestARQ_Filtro_SoAdapters(t *testing.T) {
	assert.Empty(t, violacoesFiltro(todosOsArquivos(t)))
}

// ---------------------------------------------------------------------------
// As regras detectam violações (fontes sintéticas)
// ---------------------------------------------------------------------------

func TestARQ_Allowlists_DetectamViolacoes(t *testing.T) {
	ruim := fonteSintetica(t, modShared+"/catalogo/catalogoapp", "casos.go", `package catalogoapp

import (
	"context"
	"errors"
	"fmt"
	k "github.com/rotaperfumes/shared/kernel"
)

var ErrOk = k.NovoErro(k.KindInvalido, "mensagem fixa")

var ErrConcat = k.NovoErro(k.KindInvalido, "a" + "b")

var msg = "x"
var ErrVariavel = k.NovoErro(k.KindInvalido, msg)
var ErrKindDinamico = k.NovoErro(k.Kind(3), "x")

func f(ctx context.Context, err error) error {
	_, _ = k.NovoPrincipal(1, k.RoleAdmin)
	_, _ = k.EscopoDoVendedor(7)
	_ = k.EscopoSemAcesso()
	_ = context.WithValue(ctx, keyRole, "admin")
	e := k.NovoErro(k.KindInvalido, fmt.Sprintf("id=%d", 1))
	return errors.Join(err, e)
}
`)
	casos := []struct {
		nome  string
		regra func([]arquivo) []string
		quant int
	}{
		{"NovoPrincipal fora do httpx", violacoesNovoPrincipal, 1},
		{"Escopo* fora do autorizador", violacoesEscopo, 2},
		// concatenação, variável, Kind dinâmico, e em função (fora de var + Sprintf)
		{"NovoErro não literal", violacoesNovoErro, 5},
		{"errors.Join em contexto", violacoesErrorsJoin, 1},
		{"chave legada fora do middleware", violacoesChavesLegadas, 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Len(t, c.regra([]arquivo{ruim}), c.quant)
		})
	}

	adapter := fonteSintetica(t, modShared+"/vendas/vendasmysql", "pedidos.go", `package vendasmysql

import "github.com/rotaperfumes/shared/kernel"

type Pedidos struct{}

func (r *Pedidos) Listar(e kernel.Escopo, p Pedido) {
	if e.Irrestrito() { }
	_ = e.VendedorID()
	_ = p.VendedorID() // entidade, não escopo: permitido
	var outro kernel.Escopo
	_ = outro.VendedorID()
	_, _, _ = e.Filtro()
}
`)
	assert.Len(t, violacoesFiltro([]arquivo{adapter}), 3)

	bom := fonteSintetica(t, pkgHttpx, "auth.go", `package httpx

import (
	"context"
	"github.com/rotaperfumes/shared/kernel"
)

var ErrX = kernel.NovoErro(kernel.KindNaoEncontrado, "x não encontrado")

func f(ctx context.Context) {
	_, _ = kernel.NovoPrincipal(1, kernel.RoleAdmin)
	_ = context.WithValue(ctx, chavePrincipal{}, 1)
}
`)
	for _, regra := range []func([]arquivo) []string{violacoesNovoPrincipal, violacoesEscopo, violacoesNovoErro, violacoesErrorsJoin, violacoesChavesLegadas} {
		assert.Empty(t, regra([]arquivo{bom}))
	}
}

// Dentro do próprio kernel, NovoErro é chamado sem qualificador.
func TestARQ_NovoErro_DentroDoKernel(t *testing.T) {
	src := `package kernel

var ErrBom = NovoErro(KindInterno, "erro interno")

func f() error { return NovoErro(Kind(9), "x") }
`
	a := fonteSintetica(t, pkgKernel, "erros.go", src)
	v := violacoesNovoErro([]arquivo{a})
	assert.Len(t, v, 2, "fora de var + Kind não constante: %v", v)
	assert.Contains(t, strings.Join(v, "\n"), "erros.go:"+strconv.Itoa(5))
}

// N-5: o middleware JWT legado só é referenciado pelo Autenticador.
func TestARQ_MiddlewareAuthLegado_SoAutenticador(t *testing.T) {
	arqs := todosOsArquivos(t)
	assert.Empty(t, violacoesMiddlewareAuthLegado(arqs))

	// Sanidade: a referência permitida continua sendo vista.
	vistas := 0
	for _, a := range arqs {
		referenciasA(a, modAPI+"/middleware", func(nome string, _ *ast.SelectorExpr) {
			if nome == "JWTMiddlewareWithUserCheck" {
				vistas++
			}
		})
	}
	assert.Equal(t, 1, vistas, "internal/httpx/auth.go referencia JWTMiddlewareWithUserCheck")
}

// N-4/N-5: referências sem chamada (valor de função) também são violações.
func TestARQ_Allowlists_ReferenciaSemChamada(t *testing.T) {
	ruim := fonteSintetica(t, modAPI+"/handlers", "handlers/x.go", `package handlers

import (
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/kernel"
)

var (
	novo   = kernel.NovoPrincipal
	escopo = kernel.EscopoSemAcesso
	jwt    = middleware.JWTMiddleware
)

func f() {
	_ = middleware.JWTMiddlewareWithUserCheck(nil, nil, true, false)
	_ = middleware.RequireAdmin()
	g(kernel.EscopoDoVendedor)
}
`)
	assert.Len(t, violacoesNovoPrincipal([]arquivo{ruim}), 1)
	assert.Len(t, violacoesEscopo([]arquivo{ruim}), 2)
	assert.Len(t, violacoesMiddlewareAuthLegado([]arquivo{ruim}), 3)

	bom := fonteSintetica(t, pkgHttpx, "internal/httpx/auth.go", `package httpx

import (
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/kernel"
)

var novo = kernel.NovoPrincipal

func f() { _ = middleware.JWTMiddlewareWithUserCheck(nil, nil, true, false) }
`)
	assert.Empty(t, violacoesNovoPrincipal([]arquivo{bom}))
	assert.Empty(t, violacoesMiddlewareAuthLegado([]arquivo{bom}))

	// O mesmo código em outro arquivo do httpx não é permitido.
	outro := fonteSintetica(t, pkgHttpx, "internal/httpx/router.go", `package httpx

import "github.com/rotaperfumes/rotaperfumes-api/middleware"

var m = middleware.RequireAdmin
`)
	assert.Len(t, violacoesMiddlewareAuthLegado([]arquivo{outro}), 1)
}
