package arquitetura_test

// ARQ-03 (ressalva #2 do 🟣 SecBrain): métodos de identidadeapp.Casos que não
// passam por Principal nem pela sequência completa do caso de uso.
//   - ResetarSenhasAtivos (reset em massa, CLI sem Principal, O5): só em
//     cmd/resetsenhas.
//   - RegistrarReuso / DesfazerReuso (passos do I-07, exportados para a suíte
//     SEC-12 exercitar janela e relógio): só dentro de identidadeapp.
// A varredura pega qualquer seletor com esses nomes (chamado ou não, N-4), nos
// dois módulos; _test.go fica fora.

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

var allowlistIdentidade = map[string]map[string]bool{
	"ResetarSenhasAtivos": {modAPI + "/cmd/resetsenhas": true},
	"RegistrarReuso":      {modShared + "/identidade/identidadeapp": true},
	"DesfazerReuso":       {modShared + "/identidade/identidadeapp": true},
}

func violacoesIdentidadeRestrita(arqs []arquivo) []string {
	var out []string
	for _, a := range arqs {
		ast.Inspect(a.f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if permitidos, restrito := allowlistIdentidade[sel.Sel.Name]; restrito && !permitidos[a.pkg] {
				out = append(out, a.pos(sel.Pos())+": "+sel.Sel.Name+" fora da allowlist")
			}
			return true
		})
	}
	return out
}

func TestARQ03_MetodosRestritosDeIdentidade(t *testing.T) {
	arqs := todosOsArquivos(t)
	assert.Empty(t, violacoesIdentidadeRestrita(arqs))

	// Sanidade: os usos permitidos são vistos (a varredura funciona).
	vistos := map[string]int{}
	for _, a := range arqs {
		ast.Inspect(a.f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if _, restrito := allowlistIdentidade[sel.Sel.Name]; restrito {
					vistos[sel.Sel.Name]++
				}
			}
			return true
		})
	}
	for nome := range allowlistIdentidade {
		assert.Positive(t, vistos[nome], "a varredura deveria ver %s no pacote permitido", nome)
	}
}

func TestARQ03_MetodosRestritosDeIdentidade_DetectaViolacoes(t *testing.T) {
	ruim := fonteSintetica(t, modAPI+"/internal/identidadehttp", "auth_handler.go", `package identidadehttp

func f(c *Casos) {
	_, _ = c.ResetarSenhasAtivos(nil, false)
	g := c.RegistrarReuso
	_ = c.DesfazerReuso(nil, 1, t0)
	_ = g
}
`)
	assert.Len(t, violacoesIdentidadeRestrita([]arquivo{ruim}), 3)

	bom := fonteSintetica(t, modAPI+"/cmd/resetsenhas", "main.go", `package main

func f(c *Casos) { _, _ = c.ResetarSenhasAtivos(nil, true) }
`)
	assert.Empty(t, violacoesIdentidadeRestrita([]arquivo{bom}))
}
