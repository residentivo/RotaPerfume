package arquitetura_test

// CHORE-03 (Lote 25): o parse permissivo de {id} saiu. Em internal/ não
// pode existir IDDoPathPermissivo (declaração ou uso) nem um parse
// numérico do strconv (ParseInt/ParseUint/Atoi) sobre r.PathValue — direto
// ou por uma variável atribuída de PathValue na mesma função. O caminho
// único é httpx.IDDoPath / httpx.ParseID (estrito).

import (
	"go/ast"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const pkgInternal = modAPI + "/internal"

var chore03ParsesNumericos = map[string]bool{"ParseInt": true, "ParseUint": true, "Atoi": true, "ParseFloat": true}

// ehPathValue reconhece `x.PathValue(...)`.
func ehPathValue(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "PathValue"
}

func violacoesParsePermissivo(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		if !dentroDe(a.pkg, pkgInternal) {
			continue
		}
		ast.Inspect(a.f, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == "IDDoPathPermissivo" {
				out = append(out, a.pos(id.Pos())+": IDDoPathPermissivo")
			}
			return true
		})
		alias := nomesDoImport(a.f, "strconv")
		if alias == "" {
			continue
		}
		for _, decl := range a.f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			doPath := map[string]bool{} // variáveis atribuídas de PathValue
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.AssignStmt:
					for i, rhs := range s.Rhs {
						if i < len(s.Lhs) && ehPathValue(rhs) {
							if id, ok := s.Lhs[i].(*ast.Ident); ok {
								doPath[id.Name] = true
							}
						}
					}
				case *ast.ValueSpec:
					for i, v := range s.Values {
						if i < len(s.Names) && ehPathValue(v) {
							doPath[s.Names[i].Name] = true
						}
					}
				case *ast.CallExpr:
					nome, ok := chamadaDe(s, alias, false)
					if !ok || !chore03ParsesNumericos[nome] || len(s.Args) == 0 {
						return true
					}
					arg := s.Args[0]
					if inner, ok := arg.(*ast.CallExpr); ok && len(inner.Args) == 1 {
						// strings.TrimSpace(r.PathValue(...)) e afins
						if sel, ok := inner.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name != "PathValue" {
							arg = inner.Args[0]
						}
					}
					id, ehIdent := arg.(*ast.Ident)
					if ehPathValue(arg) || (ehIdent && doPath[id.Name]) {
						out = append(out, a.pos(s.Pos())+": strconv."+nome+" sobre PathValue")
					}
				}
				return true
			})
		}
	}
	return out
}

func TestCHORE03_SemParsePermissivoEmInternal(t *testing.T) {
	arqs := todosOsArquivos(t)
	assert.Empty(t, violacoesParsePermissivo(arqs))

	// Sanidade: a varredura enxerga os handlers de internal/ e o IDDoPath.
	handlers, idDoPath := 0, 0
	for _, a := range arqs {
		if !dentroDe(a.pkg, pkgInternal) {
			continue
		}
		if strings.HasSuffix(a.caminho, "_handler.go") {
			handlers++
		}
		chamadasA(a, pkgHttpx, func(nome string, _ *ast.CallExpr) {
			if nome == "IDDoPath" || nome == "IDOuErro" {
				idDoPath++
			}
		})
	}
	assert.GreaterOrEqual(t, handlers, 10, "a varredura deveria ver os handlers")
	assert.GreaterOrEqual(t, idDoPath, 10, "os handlers leem o {id} por IDDoPath/IDOuErro")
}

func TestCHORE03_SemParsePermissivo_DetectaViolacoes(t *testing.T) {
	ruim := fonteSintetica(t, modAPI+"/internal/catalogohttp", "produto_handler.go", `package catalogohttp

import (
	"strconv"
	"strings"
)

func IDDoPathPermissivo(r *R, nome string) (int64, error) { return 0, nil }

func a(r *R) { _, _ = strconv.ParseInt(r.PathValue("id"), 10, 64) }
func b(r *R) { _, _ = strconv.Atoi(strings.TrimSpace(r.PathValue("id"))) }
func c(r *R) {
	bruto := r.PathValue("id")
	_, _ = strconv.ParseUint(bruto, 10, 64)
}
func d(r *R) {
	var v = r.PathValue("id")
	_, _ = strconv.ParseInt(v, 10, 64)
}
func e(r *R) { _, _ = httpx.IDDoPathPermissivo(r, "id") }
`)
	assert.Len(t, violacoesParsePermissivo([]arquivo{ruim}), 6)

	bom := fonteSintetica(t, modAPI+"/internal/vendashttp", "dto.go", `package vendashttp

import "strconv"

func f(r *R) {
	_, _ = httpx.IDDoPath(r, "id")
	_, _ = strconv.ParseInt(r.URL.Query().Get("pedido_id"), 10, 64)
	bruto := r.PathValue("user_id")
	_, _ = httpx.ParseID(bruto)
}
`)
	assert.Empty(t, violacoesParsePermissivo([]arquivo{bom}))

	// Fora de internal/ a regra não se aplica.
	fora := fonteSintetica(t, modShared+"/kernel", "x.go", `package kernel

import "strconv"

func f(r *R) { _, _ = strconv.ParseInt(r.PathValue("id"), 10, 64) }
`)
	assert.Empty(t, violacoesParsePermissivo([]arquivo{fora}))
}
