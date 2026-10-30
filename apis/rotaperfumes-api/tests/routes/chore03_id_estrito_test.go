package routes_test

// CHORE-03 (Lote 25): parse ESTRITO do {id} em TODAS as rotas. A tabela vem
// do router registrado em runtime (bootstrap.NovoRouter(...).Rotas()), então
// rota nova com {id} entra no teste sozinha. Zero, negativo, sinal, espaço,
// decimal, não numérico e overflow → 400 "id inválido" antes do escopo e de
// qualquer SQL (o sqlmock não tem expectativas: um SQL viraria 500). No
// DELETE /api/vendedores/{id}/clientes/{clienteId}, o {clienteId} inválido
// responde "cliente id inválido".

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
)

// chore03IDsInvalidos: o valor cru no path (já escapado para a URL).
var chore03IDsInvalidos = []struct{ nome, path string }{
	{"zero", "0"},
	{"negativo", "-1"},
	{"sinal +", "+1"},
	{"não numérico", "abc"},
	{"decimal", "1.5"},
	{"overflow int64", "9223372036854775808"},
	{"espaço antes", "%201"},
	{"espaço depois", "1%20"},
	{"hexadecimal", "0x1"},
	{"notação científica", "1e3"},
	{"zeros negativos", "-0"},
	{"dígito não ASCII", "%D9%A1"}, // "١" (árabe-índico)
}

func chore03RotasComID(t *testing.T) []httpx.Rota {
	t.Helper()
	var out []httpx.Rota
	for _, r := range arq00RotasRegistradas(t) {
		if strings.Contains(r.Padrao, "{id}") {
			out = append(out, r)
		}
	}
	return out
}

func chore03Body(metodo string) string {
	if metodo == http.MethodPut || metodo == http.MethodPost || metodo == http.MethodPatch {
		return `{}`
	}
	return ""
}

// Inventário: as 29 rotas com {id} de internal/bootstrap/rotas.go.
func TestCHORE03_InventarioDasRotasComID(t *testing.T) {
	rotas := chore03RotasComID(t)
	padroes := make([]string, 0, len(rotas))
	for _, r := range rotas {
		padroes = append(padroes, r.Metodo+" "+r.Padrao)
	}
	assert.Len(t, rotas, 29, "rota com {id} nova ou removida: revise o CHORE-03 (%v)", padroes)
}

func TestCHORE03_TodasAsRotasComID_400IdInvalidoSemSQL(t *testing.T) {
	for _, r := range chore03RotasComID(t) {
		papeis := []struct {
			nome string
			uid  int64
		}{{"admin", 1}}
		if r.Policy == httpx.Authenticated {
			// No perfil restrito o escopo (SQL) viria antes se o parse estivesse atrasado.
			papeis = append(papeis, struct {
				nome string
				uid  int64
			}{"normal", 2})
		}
		for _, papel := range papeis {
			for _, id := range chore03IDsInvalidos {
				path := strings.ReplaceAll(r.Padrao, "{id}", id.path)
				path = strings.ReplaceAll(path, "{clienteId}", "10")
				t.Run(r.Metodo+" "+r.Padrao+"/"+papel.nome+"/"+id.nome, func(t *testing.T) {
					h, mock := arq00Router(t, fakeUserStatusChecker{})
					w := serve(h, r.Metodo, path, token(t, papel.uid, papel.nome), chore03Body(r.Metodo))
					require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
					assert.JSONEq(t, `{"success":false,"error":"id inválido"}`, w.Body.String())
					assert.NoError(t, mock.ExpectationsWereMet())
				})
			}
		}
	}
}

// {clienteId} inválido com {id} válido: 400 "cliente id inválido" sem SQL.
func TestCHORE03_DesvincularCliente_ClienteIDInvalido(t *testing.T) {
	for _, id := range chore03IDsInvalidos {
		t.Run(id.nome, func(t *testing.T) {
			h, mock := arq00Router(t, fakeUserStatusChecker{})
			w := serve(h, http.MethodDelete, "/api/vendedores/1/clientes/"+id.path, token(t, 1, "admin"), "")
			require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
			assert.JSONEq(t, `{"success":false,"error":"cliente id inválido"}`, w.Body.String())
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
	// {id} inválido tem precedência sobre o {clienteId}.
	h, _ := arq00Router(t, fakeUserStatusChecker{})
	w := serve(h, http.MethodDelete, "/api/vendedores/0/clientes/abc", token(t, 1, "admin"), "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"success":false,"error":"id inválido"}`, w.Body.String())
}

// Controle positivo: ids válidos (inclusive o maior int64 e zeros à
// esquerda) passam do parse e chegam ao caso de uso (que falha no banco
// sem expectativas: nunca o 400 "id inválido").
func TestCHORE03_IDsValidos_PassamDoParse(t *testing.T) {
	validos := []string{"1", "42", "007", "9223372036854775807"}
	for _, r := range chore03RotasComID(t) {
		for _, id := range validos {
			path := strings.ReplaceAll(r.Padrao, "{id}", id)
			path = strings.ReplaceAll(path, "{clienteId}", "10")
			t.Run(r.Metodo+" "+r.Padrao+"/"+id, func(t *testing.T) {
				h, _ := arq00Router(t, fakeUserStatusChecker{})
				w := serve(h, r.Metodo, path, token(t, 1, "admin"), chore03Body(r.Metodo))
				assert.NotContains(t, w.Body.String(), `"id inválido"`, "status %d", w.Code)
			})
		}
	}
}
