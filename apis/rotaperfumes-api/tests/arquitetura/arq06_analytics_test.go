package arquitetura_test

// ARQ-06 (Analytics, D-02 e O1), nos pacotes de produção (_test.go fora):
//   - TestARQ_SprintfSQLSoConstantes (§4.3, adiado do ARQ-01): em
//     analyticsmysql, todo fmt.Sprintf tem o formato constante e cada
//     argumento é constante (literal ou const do pacote) ou a chamada de uma
//     função do próprio pacote que faz switch sobre um parâmetro do enum
//     analytics.<Tipo> e só devolve constantes. Nada que venha de parâmetro,
//     variável ou outra função chega ao texto do SQL;
//   - toda consulta exportada dos adapters analyticsmysql recebe
//     kernel.Escopo (o "0 = global" do dashboard legado some, E-02).
// Cada regra tem o controle negativo com fonte sintética.

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	pkgAnalytics      = modShared + "/analytics"
	pkgAnalyticsMySQL = modShared + "/analytics/analyticsmysql"
)

// declaracoesDoPacote junta as const e as funções (sem receptor) de nível de
// pacote de todos os arquivos do pacote alvo, com o arquivo de cada função.
func declaracoesDoPacote(arqs []arquivo, pkg string) (consts map[string]bool, funcs map[string]*ast.FuncDecl, arqDe map[string]arquivo) {
	consts, funcs, arqDe = map[string]bool{}, map[string]*ast.FuncDecl{}, map[string]arquivo{}
	for _, a := range arqs {
		if a.pkg != pkg {
			continue
		}
		for _, d := range a.f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok != token.CONST {
					continue
				}
				for _, s := range d.Specs {
					for _, n := range s.(*ast.ValueSpec).Names {
						consts[n.Name] = true
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil {
					funcs[d.Name.Name] = d
					arqDe[d.Name.Name] = a
				}
			}
		}
	}
	return consts, funcs, arqDe
}

// constante: literal string ou const de nível de pacote.
func constante(e ast.Expr, consts map[string]bool) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.Ident:
		return consts[e.Name]
	}
	return false
}

// switchSobreEnum: a função tem um parâmetro do tipo analytics.<Tipo>, faz
// switch sobre ele e todo return devolve uma única constante.
func switchSobreEnum(fn *ast.FuncDecl, a arquivo, consts map[string]bool) bool {
	alias := nomesDoImport(a.f, pkgAnalytics)
	params := map[string]bool{}
	for _, campo := range fn.Type.Params.List {
		if sel, ok := campo.Type.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && alias != "" && x.Name == alias {
				for _, n := range campo.Names {
					params[n.Name] = true
				}
			}
		}
	}
	temSwitch, soConstantes := false, true
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SwitchStmt:
			if tag, ok := n.Tag.(*ast.Ident); ok && params[tag.Name] {
				temSwitch = true
			}
		case *ast.ReturnStmt:
			if len(n.Results) != 1 || !constante(n.Results[0], consts) {
				soConstantes = false
			}
		}
		return true
	})
	return temSwitch && soConstantes
}

// violacoesSprintfSQL devolve as violações e quantos Sprintf foram vistos.
func violacoesSprintfSQL(arqs []arquivo) (violacoes []string, vistos int) {
	consts, funcs, arqDe := declaracoesDoPacote(arqs, pkgAnalyticsMySQL)
	argOK := func(e ast.Expr) bool {
		if constante(e, consts) {
			return true
		}
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok || funcs[id.Name] == nil {
			return false
		}
		return switchSobreEnum(funcs[id.Name], arqDe[id.Name], consts)
	}
	for _, a := range arqs {
		if a.pkg != pkgAnalyticsMySQL {
			continue
		}
		chamadasA(a, "fmt", func(nome string, call *ast.CallExpr) {
			if nome != "Sprintf" {
				return
			}
			vistos++
			if len(call.Args) == 0 || !constante(call.Args[0], consts) {
				violacoes = append(violacoes, a.pos(call.Pos())+": formato do Sprintf não é constante")
				return
			}
			for _, arg := range call.Args[1:] {
				if !argOK(arg) {
					violacoes = append(violacoes, a.pos(arg.Pos())+": argumento do Sprintf não é constante nem switch sobre enum")
				}
			}
		})
	}
	return violacoes, vistos
}

// violacoesConsultaSemEscopo: todo método exportado dos adapters
// analyticsmysql recebe kernel.Escopo.
func violacoesConsultaSemEscopo(arqs []arquivo) (violacoes []string, consultas int) {
	for _, a := range arqs {
		if a.pkg != pkgAnalyticsMySQL {
			continue
		}
		alias := nomesDoImport(a.f, pkgKernel)
		for _, d := range a.f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			consultas++
			if !recebeEscopo(fn, alias) {
				violacoes = append(violacoes, a.pos(fn.Pos())+": "+fn.Name.Name+" sem parâmetro kernel.Escopo")
			}
		}
	}
	return violacoes, consultas
}

func TestARQ_SprintfSQLSoConstantes(t *testing.T) {
	violacoes, vistos := violacoesSprintfSQL(todosOsArquivos(t))
	assert.Empty(t, violacoes)
	assert.GreaterOrEqual(t, vistos, 2, "VendasTotais e TotalPedidos montam o período por Sprintf")
}

func TestARQ06_ConsultasAnalyticsRecebemEscopo(t *testing.T) {
	violacoes, consultas := violacoesConsultaSemEscopo(todosOsArquivos(t))
	assert.Empty(t, violacoes)
	assert.GreaterOrEqual(t, consultas, 12, "7 consultas do Dashboard e 5 de Clientes")
}

func TestARQ06_RegrasAnalytics_DetectamViolacoes(t *testing.T) {
	const cabecalho = `package analyticsmysql
import (
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/kernel"
)
const base = "SELECT %s"
const mes = "MONTH(x)"
`
	ruim := fonteSintetica(t, pkgAnalyticsMySQL, "ruim.go", cabecalho+`
func clausula(p analytics.Periodo) string {
	switch p {
	case analytics.PeriodoHoje:
		return "DATE(x)"
	default:
		return mes
	}
}
func semSwitch(p analytics.Periodo) string { return mes }
func devolveParam(p analytics.Periodo, s string) string {
	switch p {
	default:
		return s
	}
}
func switchSobreOutro(p analytics.Periodo, s string) string {
	switch s {
	default:
		return mes
	}
}
func f(p analytics.Periodo, coluna string) {
	formato := base
	_ = fmt.Sprintf(formato, mes)                    // formato variável
	_ = fmt.Sprintf(base, coluna)                    // parâmetro
	_ = fmt.Sprintf(base, semSwitch(p))              // função sem switch
	_ = fmt.Sprintf(base, devolveParam(p, coluna))   // switch que devolve parâmetro
	_ = fmt.Sprintf(base, switchSobreOutro(p, ""))   // switch fora do enum
	_ = fmt.Sprintf(base, strings.ToUpper(coluna))   // função de outro pacote
	_ = fmt.Sprintf(base, clausula(p)+coluna)        // concatenação
	_ = fmt.Sprintf(base, clausula(p))               // ok
	_ = fmt.Sprintf("SELECT %s %s", "x", mes)        // ok
}
type R struct{}
func (R) Total(ctx any, vendedorID int64) {}
func (R) Ranking(ctx any, e kernel.Escopo) {}
func (R) interno(ctx any) {}
`)
	violacoes, vistos := violacoesSprintfSQL([]arquivo{ruim})
	assert.Len(t, violacoes, 7, "%v", violacoes)
	assert.Equal(t, 9, vistos)

	semEscopo, consultas := violacoesConsultaSemEscopo([]arquivo{ruim})
	assert.Len(t, semEscopo, 1)
	assert.Equal(t, 2, consultas, "métodos não exportados ficam fora")

	// Outro pacote não é analisado (a regra é do analyticsmysql).
	fora := fonteSintetica(t, modShared+"/vendas/vendasmysql", "x.go", "package vendasmysql\nimport \"fmt\"\nfunc f(s string) { _ = fmt.Sprintf(s) }\n")
	v, n := violacoesSprintfSQL([]arquivo{fora})
	assert.Empty(t, v)
	assert.Zero(t, n)
}
