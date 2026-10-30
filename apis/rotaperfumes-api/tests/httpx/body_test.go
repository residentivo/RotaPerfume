package httpx_test

// ARQ-02: kit de body/path/paginação mapeada que nasceu com o Catálogo.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/kernel"
)

func TestDecodificarJSON(t *testing.T) {
	type dto struct {
		N int `json:"n"`
	}
	casos := []struct {
		body string
		ok   bool
		n    int
	}{
		{`{"n":3}`, true, 3},
		{`{"n":3,"extra":"ignorado"}`, true, 3},
		{`{"n":3} lixo depois`, true, 3}, // como o json.Decoder legado: só o 1º valor
		{`{invalido`, false, 0},
		{`{"n":"3"}`, false, 0},
		{``, false, 0},
	}
	for _, c := range casos {
		t.Run(c.body, func(t *testing.T) {
			var d dto
			err := httpx.DecodificarJSON(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(c.body)), &d)
			if !c.ok {
				assert.ErrorIs(t, err, httpx.ErrBodyJSONInvalido)
				assert.Equal(t, "body JSON inválido", err.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.n, d.N)
		})
	}
}

func TestLerAtivoOpcional(t *testing.T) {
	sim, nao := true, false
	casos := []struct {
		nome string
		body string
		want *bool
		err  bool
	}{
		{"vazio = toggle", "", nil, false},
		{"null = toggle", "null", nil, false},
		{"objeto vazio = toggle", "{}", nil, false},
		{"ativo null = toggle", `{"ativo":null}`, nil, false},
		{"true", `{"ativo":true}`, &sim, false},
		{"false", `{"ativo":false}`, &nao, false},
		{"tipo errado", `{"ativo":"false"}`, nil, true},
		{"número", `{"ativo":1}`, nil, true},
		{"malformado", `{"ativo":`, nil, true},
		{"lixo após o objeto", `{"ativo":true} x`, nil, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got, err := httpx.LerAtivoOpcional(httptest.NewRequest(http.MethodPatch, "/", strings.NewReader(c.body)))
			if c.err {
				assert.ErrorIs(t, err, httpx.ErrBodyJSONInvalido)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}

	t.Run("sem body (nil) = toggle", func(t *testing.T) {
		got, err := httpx.LerAtivoOpcional(&http.Request{Body: nil})
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

// TestIDOuErro_Estrito (CHORE-03): o IDOuErro usa o parse estrito de
// IDDoPath; o parse permissivo (IDDoPathPermissivo) foi removido. Só
// dígitos, > 0 e cabendo em int64; o resto responde 400 "id inválido".
func TestIDOuErro_Estrito(t *testing.T) {
	casos := []struct {
		v  string
		id int64
		ok bool
	}{
		{"1", 1, true},
		{"0", 0, false},
		{"-5", 0, false},
		{"+7", 0, false},
		{" 1", 0, false},
		{"1.5", 0, false},
		{"abc", 0, false},
		{"", 0, false},
		{"99999999999999999999", 0, false},
	}
	for _, c := range casos {
		t.Run(c.v, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.SetPathValue("id", c.v)
			w := httptest.NewRecorder()
			id, ok := httpx.IDOuErro(w, r, "[teste]", "id")
			assert.Equal(t, c.ok, ok)
			if c.ok {
				assert.Equal(t, c.id, id)
				return
			}
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "id inválido")
			_, err := httpx.IDDoPath(r, "id")
			assert.ErrorIs(t, err, httpx.ErrIDInvalido)
		})
	}
}

func TestJSONPaginadoCom(t *testing.T) {
	dobro := func(n int) int { return 2 * n }

	w := httptest.NewRecorder()
	httpx.JSONPaginadoCom(w, kernel.NovoResultado([]int{1, 2}, kernel.NovaPagina(2, 2), 5), dobro)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"data":[2,4],"pagination":{"limit":2,"page":2,"pages":3,"total":5},"success":true}`+"\n", w.Body.String())

	w = httptest.NewRecorder()
	httpx.JSONPaginadoCom(w, kernel.NovoResultado[int](nil, kernel.Pagina{}, 0), dobro)
	corpo, _ := io.ReadAll(w.Body)
	assert.Equal(t, `{"data":null,"pagination":{"limit":20,"page":1,"pages":0,"total":0},"success":true}`+"\n", string(corpo),
		"sem itens: data null, como o writeJSONWithPagination legado")
}
