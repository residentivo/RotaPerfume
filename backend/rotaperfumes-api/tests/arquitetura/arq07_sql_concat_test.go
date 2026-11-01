package arquitetura_test

// ARQ-07 (pendência k, sugestão do 🔴 TestBrain no ARQ-06): o
// TestARQ_SprintfSQLSoConstantes cobre o fmt.Sprintf; este teste cobre a
// CONCATENAÇÃO com `+` nos textos de SQL passados a QueryContext,
// QueryRowContext e ExecContext em analyticsmysql (o pacote em que o SQL é
// montado por pedaços, D-02). Cada operando tem de ser:
//   - literal string ou const do pacote;
//   - chamada a função/método do próprio pacote (cláusulas por switch sobre
//     enum, marcadores "?,?", filtros de escopo por Filtro(), O1) ou o
//     fmt.Sprintf já conferido pela regra do Sprintf;
//   - variável local que só recebe operandos assim;
//   - parâmetro de função NÃO exportada do pacote cujas chamadas, todas no
//     pacote, passam operandos assim (interprocedural, um nível por vez).
// Parâmetro de método/função exportada (valor vindo de fora) ou chamada a
// função de outro pacote viram violação. Limite conhecido: o corpo das
// funções do pacote chamadas como operando não é reanalisado aqui.

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var metodosSQL = map[string]bool{"QueryContext": true, "QueryRowContext": true, "ExecContext": true}

// funcaoSQL é uma função ou método do pacote com os parâmetros indexados.
type funcaoSQL struct {
	decl      *ast.FuncDecl
	params    map[string]int // nome → posição
	exportada bool
}

type analiseConcat struct {
	consts  map[string]bool
	funcs   map[string]funcaoSQL       // por nome (funções e métodos)
	chamada map[string][]*ast.CallExpr // chamadas no pacote, por nome
}

func novaAnaliseConcat(arqs []arquivo, pkg string) *analiseConcat {
	a := &analiseConcat{consts: map[string]bool{}, funcs: map[string]funcaoSQL{}, chamada: map[string][]*ast.CallExpr{}}
	for _, arq := range arqs {
		if arq.pkg != pkg {
			continue
		}
		for _, d := range arq.f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok == token.CONST {
					for _, s := range d.Specs {
						for _, n := range s.(*ast.ValueSpec).Names {
							a.consts[n.Name] = true
						}
					}
				}
			case *ast.FuncDecl:
				f := funcaoSQL{decl: d, params: map[string]int{}, exportada: ast.IsExported(d.Name.Name)}
				i := 0
				for _, campo := range d.Type.Params.List {
					for _, n := range campo.Names {
						f.params[n.Name] = i
						i++
					}
					if len(campo.Names) == 0 {
						i++
					}
				}
				a.funcs[d.Name.Name] = f
			}
		}
		ast.Inspect(arq.f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if nome := nomeDaChamada(call); nome != "" {
					a.chamada[nome] = append(a.chamada[nome], call)
				}
			}
			return true
		})
	}
	return a
}

// nomeDaChamada: f(...) ou x.f(...) (método do pacote ou de outro valor).
func nomeDaChamada(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	case *ast.IndexExpr: // genérica: f[T](...)
		if id, ok := fn.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// atribuicoes devolve, para cada variável local de fn, as expressões que
// ela recebe (num := ou = com chamada de vários retornos, a própria chamada).
func atribuicoes(fn *ast.FuncDecl) map[string][]ast.Expr {
	out := map[string][]ast.Expr{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range n.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				if len(n.Rhs) == len(n.Lhs) {
					out[id.Name] = append(out[id.Name], n.Rhs[i])
				} else if len(n.Rhs) == 1 {
					out[id.Name] = append(out[id.Name], n.Rhs[0])
				}
			}
		case *ast.ValueSpec:
			for i, nome := range n.Names {
				if i < len(n.Values) {
					out[nome.Name] = append(out[nome.Name], n.Values[i])
				}
			}
		}
		return true
	})
	return out
}

// operandoOK aplica a regra do cabeçalho; visto evita laços.
func (a *analiseConcat) operandoOK(e ast.Expr, fn *ast.FuncDecl, visto map[string]bool) bool {
	switch e := e.(type) {
	case *ast.ParenExpr:
		return a.operandoOK(e.X, fn, visto)
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.BinaryExpr:
		return e.Op == token.ADD && a.operandoOK(e.X, fn, visto) && a.operandoOK(e.Y, fn, visto)
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if x, ok := sel.X.(*ast.Ident); ok && x.Name == "fmt" && sel.Sel.Name == "Sprintf" {
				return true // conferido por TestARQ_SprintfSQLSoConstantes
			}
			// método do pacote chamado num valor local (r.f); x.f com x sem
			// declaração no arquivo é um pacote importado: violação.
			_, doPacote := a.funcs[sel.Sel.Name]
			x, ehIdent := sel.X.(*ast.Ident)
			return doPacote && ehIdent && x.Obj != nil
		}
		_, ok := a.funcs[nomeDaChamada(e)]
		return ok
	case *ast.Ident:
		if a.consts[e.Name] {
			return true
		}
		chave := fn.Name.Name + "." + e.Name
		if visto[chave] {
			return true
		}
		visto[chave] = true
		if pos, ehParam := a.funcs[fn.Name.Name].params[e.Name]; ehParam {
			return a.parametroOK(fn.Name.Name, pos, visto)
		}
		exprs := atribuicoes(fn)[e.Name]
		if len(exprs) == 0 {
			return false
		}
		for _, x := range exprs {
			if !a.operandoOK(x, fn, visto) {
				return false
			}
		}
		return true
	}
	return false
}

// parametroOK: só função não exportada, e toda chamada no pacote passa um
// operando permitido naquela posição.
func (a *analiseConcat) parametroOK(nome string, pos int, visto map[string]bool) bool {
	f := a.funcs[nome]
	if f.exportada || len(a.chamada[nome]) == 0 {
		return false
	}
	for _, call := range a.chamada[nome] {
		if pos >= len(call.Args) {
			return false
		}
		chamador := a.funcaoQueContem(call)
		if chamador == nil || !a.operandoOK(call.Args[pos], chamador, visto) {
			return false
		}
	}
	return true
}

func (a *analiseConcat) funcaoQueContem(n ast.Node) *ast.FuncDecl {
	for _, f := range a.funcs {
		if f.decl.Body != nil && f.decl.Body.Pos() <= n.Pos() && n.End() <= f.decl.Body.End() {
			return f.decl
		}
	}
	return nil
}

// violacoesConcatSQL devolve as violações e quantas concatenações foram vistas.
func violacoesConcatSQL(arqs []arquivo, pkg string) (violacoes []string, vistos int) {
	a := novaAnaliseConcat(arqs, pkg)
	for _, arq := range arqs {
		if arq.pkg != pkg {
			continue
		}
		ast.Inspect(arq.f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !metodosSQL[sel.Sel.Name] || len(call.Args) < 2 {
				return true
			}
			texto := call.Args[1]
			if _, ehConcat := texto.(*ast.BinaryExpr); !ehConcat {
				if _, ehIdent := texto.(*ast.Ident); !ehIdent {
					return true
				}
			}
			fn := a.funcaoQueContem(call)
			if fn == nil {
				return true
			}
			if _, ehConcat := texto.(*ast.BinaryExpr); ehConcat {
				vistos++
			}
			if !a.operandoOK(texto, fn, map[string]bool{}) {
				violacoes = append(violacoes, arq.pos(texto.Pos())+": texto de SQL de "+sel.Sel.Name+" concatena valor que não é constante")
			}
			return true
		})
	}
	return violacoes, vistos
}

func TestARQ07_ConcatenacaoSQLSoConstantes(t *testing.T) {
	v, vistos := violacoesConcatSQL(todosOsArquivos(t), pkgAnalyticsMySQL)
	assert.Empty(t, v, strings.Join(v, "\n"))
	assert.Positive(t, vistos, "a regra deveria ver as concatenações do analyticsmysql")
}

func TestARQ07_ConcatenacaoSQL_DetectaViolacoes(t *testing.T) {
	a := fonteSintetica(t, pkgAnalyticsMySQL, "x.go", `package analyticsmysql

import "strings"

const sqlBase = "SELECT * FROM t WHERE "

type R struct{ q Q }

func (r *R) Buscar(ctx C, campo string, busca string) {
	_ = r.q.QueryContext(ctx, sqlBase+campo)          // parâmetro de método exportado
	q := sqlBase + busca
	_ = r.q.QueryRowContext(ctx, q)                    // variável com parâmetro
	_ = r.q.ExecContext(ctx, sqlBase+strings.ToUpper("x")) // outro pacote
	r.contar(ctx, campo)                               // repassa valor de fora
	r.contar(ctx, "x = 1")
}

func (r *R) contar(ctx C, where string) {
	_ = r.q.QueryRowContext(ctx, sqlBase+where) // uma chamada passa parâmetro externo
}

func (r *R) Bom(ctx C, n int) {
	filtro, _ := clausula(n)
	_ = r.q.QueryContext(ctx, sqlBase+filtro+marcadores(n)+" LIMIT 1")
	r.interno(ctx, "a = ?")
}

func (r *R) interno(ctx C, where string) {
	_ = r.q.QueryContext(ctx, sqlBase+where)
}

func clausula(n int) (string, int) { return "a = 1", n }
func marcadores(n int) string     { return strings.Repeat("?", n) }
`)
	v, vistos := violacoesConcatSQL([]arquivo{a}, pkgAnalyticsMySQL)
	assert.Equal(t, 5, vistos)
	assert.Len(t, v, 4, strings.Join(v, "\n"))
}
