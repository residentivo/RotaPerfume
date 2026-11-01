package arquitetura_test

// ARQ-01 (docs/arquitetura-ddd.md §4.1/§4.2): regras de dependência entre as
// camadas do módulo shared. Pacotes que ainda não existem (contextos dos
// lotes ARQ-02..07) são simplesmente ignorados, então as regras valem desde
// o primeiro lote.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// contextos são os bounded contexts previstos no blueprint (§2.1).
var contextos = []string{"catalogo", "identidade", "comercial", "vendas", "analytics", "notificacao"}

var legado = []string{mod + "/models", mod + "/repositories", mod + "/services"}

const modAPI = "github.com/rotaperfumes/rotaperfumes-api"

type regra struct {
	nome      string
	pacote    string   // import path exato analisado
	proibidos []string // import exato ou prefixo de subpacote
}

func dentroDe(imp, prefixo string) bool {
	return imp == prefixo || strings.HasPrefix(imp, prefixo+"/")
}

func regras() []regra {
	rs := []regra{{
		nome:      "kernel é puro (só stdlib, sem sql/http/log)",
		pacote:    mod + "/kernel",
		proibidos: []string{"database/sql", "net/http", "log", mod, "github.com", "golang.org"},
	}}
	for _, ctx := range contextos {
		base := mod + "/" + ctx
		rs = append(rs,
			regra{
				nome:   "domínio " + ctx + " sem infra nem transporte",
				pacote: base,
				proibidos: append([]string{"database/sql", "net/http", "encoding/json", "log",
					mod + "/config", mod + "/vlog", mod + "/db", base + "/" + ctx + "app", base + "/" + ctx + "mysql"}, legado...),
			},
			regra{
				nome:   "aplicação " + ctx + "app sem sql/http/config/adapters/infra",
				pacote: base + "/" + ctx + "app",
				proibidos: append([]string{"database/sql", "net/http", mod + "/config", mod + "/adapters",
					mod + "/db", base + "/" + ctx + "mysql", modAPI}, legado...),
			},
			regra{
				nome:      "infra " + ctx + "mysql não importa a aplicação nem o transporte",
				pacote:    base + "/" + ctx + "mysql",
				proibidos: append([]string{base + "/" + ctx + "app", "net/http", modAPI}, legado...),
			},
		)
	}
	return rs
}

// contextoDe devolve o contexto se ip é um pacote de contexto (domínio, app
// ou infra), e "" caso contrário.
func contextoDe(ip string) string {
	for _, ctx := range contextos {
		if dentroDe(ip, mod+"/"+ctx) {
			return ctx
		}
	}
	return ""
}

// violacoesDeCamada aplica as regras por pacote.
func violacoesDeCamada(pacotes []pacote) []string {
	var out []string
	for _, p := range pacotes {
		for _, r := range regras() {
			if p.importPath != r.pacote {
				continue
			}
			for _, imp := range p.imports {
				for _, proibido := range r.proibidos {
					if dentroDe(imp, proibido) {
						out = append(out, fmt.Sprintf("[%s] %s importa %s", r.nome, p.importPath, imp))
						break // uma violação por import
					}
				}
			}
		}
	}
	return out
}

// violacoesEntreContextos: um contexto não importa outro (a dependência passa
// por porta declarada no próprio <ctx>app e ligada no bootstrap, §3.1); o
// domínio só importa kernel, cnpj e tz do projeto; nenhum pacote novo
// (kernel ou contexto) importa o legado.
func violacoesEntreContextos(pacotes []pacote) []string {
	var out []string
	for _, p := range pacotes {
		ctx := contextoDe(p.importPath)
		novo := ctx != "" || p.importPath == mod+"/kernel"
		for _, imp := range p.imports {
			if novo {
				for _, l := range legado {
					if dentroDe(imp, l) {
						out = append(out, fmt.Sprintf("[legado] %s importa %s", p.importPath, imp))
					}
				}
			}
			if ctx == "" || !strings.HasPrefix(imp, mod+"/") {
				continue
			}
			if outro := contextoDe(imp); outro != "" && outro != ctx {
				out = append(out, fmt.Sprintf("[entre contextos] %s importa %s", p.importPath, imp))
			}
			ehDominio := p.importPath == mod+"/"+ctx
			permitido := dentroDe(imp, mod+"/kernel") || dentroDe(imp, mod+"/cnpj") || dentroDe(imp, mod+"/tz")
			if ehDominio && !permitido {
				out = append(out, fmt.Sprintf("[domínio impuro] %s importa %s", p.importPath, imp))
			}
		}
	}
	return out
}

func TestARQ_Dependencias(t *testing.T) {
	assert.Empty(t, violacoesDeCamada(pacotesDoModulo(t, raizModulo, mod)))
}

func TestARQ_Dependencias_EntreContextosELegado(t *testing.T) {
	assert.Empty(t, violacoesEntreContextos(pacotesDoModulo(t, raizModulo, mod)))
}

// As regras detectam violações (pacotes sintéticos): sem isto, um teste que
// nunca encontra nada passaria em silêncio.
func TestARQ_Dependencias_DetectaViolacoes(t *testing.T) {
	sinteticos := []pacote{
		{mod + "/kernel", []string{"strings", "time", "log"}},
		{mod + "/kernel", []string{"github.com/rotaperfumes/shared/vlog"}},
		{mod + "/catalogo", []string{"database/sql/driver", mod + "/kernel"}},
		{mod + "/catalogo", []string{mod + "/catalogo/catalogomysql"}},
		{mod + "/catalogo/catalogoapp", []string{"net/http"}},
		{mod + "/catalogo/catalogoapp", []string{mod + "/catalogo/catalogomysql"}},
		{mod + "/catalogo/catalogomysql", []string{mod + "/catalogo/catalogoapp"}},
		{mod + "/vendas/vendasapp", []string{mod + "/repositories"}},
	}
	assert.Len(t, violacoesDeCamada(sinteticos), len(sinteticos))

	cruzados := []pacote{
		{mod + "/vendas/vendasapp", []string{mod + "/comercial/comercialmysql"}}, // entre contextos
		{mod + "/catalogo", []string{mod + "/config"}},                           // domínio impuro
		{mod + "/kernel", []string{mod + "/models"}},                             // legado
	}
	assert.Len(t, violacoesEntreContextos(cruzados), len(cruzados))

	permitidos := []pacote{
		{mod + "/kernel", []string{"errors", "math", "strings", "time"}},
		{mod + "/catalogo", []string{mod + "/kernel", "strings"}},
		{mod + "/comercial", []string{mod + "/cnpj", mod + "/tz", mod + "/kernel"}},
		{mod + "/catalogo/catalogomysql", []string{mod + "/db", mod + "/catalogo", mod + "/kernel", "database/sql"}},
		{mod + "/repositories", []string{"database/sql", mod + "/models"}}, // legado fora das regras
	}
	assert.Empty(t, violacoesDeCamada(permitidos))
	assert.Empty(t, violacoesEntreContextos(permitidos))
}

// Sanidade: a varredura enxerga os pacotes que as regras precisam ver.
func TestARQ_Dependencias_VarreduraEncontraKernel(t *testing.T) {
	achou := map[string]bool{}
	for _, p := range pacotesDoModulo(t, raizModulo, mod) {
		achou[p.importPath] = true
	}
	for _, esperado := range []string{mod + "/kernel", mod + "/db", mod + "/vlog", mod + "/notificacao/notificacaosmtp", mod + "/importers/importkit"} {
		assert.True(t, achou[esperado], "varredura não encontrou %s", esperado)
	}
	// ARQ-07: o legado saiu ("≥ 1 uso" virou "0 usos"): os pacotes não existem
	// mais e nenhum pacote do módulo os importa.
	for _, l := range legado {
		assert.False(t, achou[l], "pacote legado voltou: %s", l)
	}
	for _, p := range pacotesDoModulo(t, raizModulo, mod) {
		for _, imp := range p.imports {
			for _, l := range legado {
				assert.False(t, dentroDe(imp, l), "%s importa o legado %s", p.importPath, imp)
			}
		}
	}
	assert.True(t, existe(t, raizModulo+"/go.mod"), "raiz do módulo errada")
}
