package httpx_test

// Envelope (F-05) byte a byte igual ao map[string]any legado; helpers de
// query/path (F-10, C-02, R-f).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/kernel"
)

// legadoJSON reproduz o writeJSON anterior ao ARQ-01 (map + Encoder).
func legadoJSON(data any, errMsg string) string {
	resp := map[string]any{"success": errMsg == ""}
	if data != nil {
		resp["data"] = data
	}
	if errMsg != "" {
		resp["error"] = errMsg
	}
	b, _ := json.Marshal(resp)
	return string(b) + "\n"
}

type produtoTeste struct {
	ID   int64  `json:"id"`
	Nome string `json:"nome,omitempty"`
}

func TestJSON_IgualAoLegadoByteAByte(t *testing.T) {
	var nilPtr *produtoTeste
	casos := []struct {
		nome   string
		status int
		data   any
		errMsg string
	}{
		{"sucesso com struct", 200, produtoTeste{ID: 1, Nome: "A <b> & c"}, ""},
		{"sucesso sem data", 200, nil, ""},
		{"sucesso com slice vazio", 200, []produtoTeste{}, ""},
		{"sucesso com map", 201, map[string]any{"id": 7, "ok": true}, ""},
		{"ponteiro nil tipado", 200, nilPtr, ""},
		{"erro", 404, nil, "produto não encontrado"},
		{"erro com data", 409, map[string]int{"x": 1}, "conflito"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpx.JSON(w, c.status, c.data, c.errMsg)
			assert.Equal(t, c.status, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.Equal(t, legadoJSON(c.data, c.errMsg), w.Body.String())
		})
	}
}

func TestJSONComPaginacao_IgualAoLegado(t *testing.T) {
	for _, data := range []any{[]produtoTeste{{ID: 1}}, []produtoTeste(nil), nil} {
		w := httptest.NewRecorder()
		httpx.JSONComPaginacao(w, http.StatusOK, data, httpx.Paginacao{Page: 2, Limit: 10, Total: 31, Pages: 4})
		legado, _ := json.Marshal(map[string]any{
			"success":    true,
			"data":       data,
			"pagination": map[string]int{"page": 2, "limit": 10, "total": 31, "pages": 4},
		})
		assert.Equal(t, string(legado)+"\n", w.Body.String())
	}
}

func TestJSONPaginado_DoResultado(t *testing.T) {
	w := httptest.NewRecorder()
	r := kernel.NovoResultado([]produtoTeste{{ID: 1}, {ID: 2}}, kernel.NovaPagina(2, 2), 5)
	httpx.JSONPaginado(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"success":true,"data":[{"id":1},{"id":2}],"pagination":{"page":2,"limit":2,"total":5,"pages":3}}`, w.Body.String())
}

func TestSemConteudo(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.SemConteudo(w)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestPaginaDaQuery(t *testing.T) {
	casos := []struct {
		query           string
		page, limit, of int
	}{
		{"", 1, 20, 0},
		{"page=3&limit=10", 3, 10, 20},
		{"page=abc&limit=xyz", 1, 20, 0},
		{"page=-1&limit=0", 1, 20, 0},
		{"limit=1000", 1, 100, 0},
	}
	for _, c := range casos {
		t.Run(c.query, func(t *testing.T) {
			p := httpx.PaginaDaQuery(httptest.NewRequest("GET", "/x?"+c.query, nil))
			assert.Equal(t, c.page, p.Numero())
			assert.Equal(t, c.limit, p.Limite())
			assert.Equal(t, c.of, p.Offset())
		})
	}
}

type ordemTeste string

func TestOrdenacaoDaQuery(t *testing.T) {
	padrao := kernel.NovaOrdenacao(ordemTeste("id"), kernel.Desc)
	validos := []ordemTeste{"id", "nome"}
	casos := []struct {
		query string
		campo ordemTeste
		dir   kernel.Direcao
	}{
		{"", "id", kernel.Desc},
		{"order_by=nome&order_dir=asc", "nome", kernel.Asc},
		{"order_by=senha_hash&order_dir=asc", "id", kernel.Asc},
		{"order_by=nome;DROP&order_dir=x", "id", kernel.Desc},
	}
	for _, c := range casos {
		o := httpx.OrdenacaoDaQuery(httptest.NewRequest("GET", "/x?"+c.query, nil), validos, padrao)
		assert.Equal(t, c.campo, o.Campo(), c.query)
		assert.Equal(t, c.dir, o.Direcao(), c.query)
	}
}

func TestBoolDaQuery(t *testing.T) {
	casos := map[string]*bool{"true": ptr(true), "FALSE": ptr(false), " True ": ptr(true), "": nil, "1": nil, "sim": nil}
	for v, want := range casos {
		req := httptest.NewRequest("GET", "/x", nil)
		q := req.URL.Query()
		q.Set("ativo", v)
		req.URL.RawQuery = q.Encode()
		assert.Equal(t, want, httpx.BoolDaQuery(req, "ativo"), "v=%q", v)
		assert.Equal(t, want, httpx.ParseBoolOpcional(v), "v=%q", v)
	}
}

func ptr(b bool) *bool { return &b }

// R-f: obrigatório, decimal, > 0 e sem "/".
func TestParseID_Estrito(t *testing.T) {
	casos := []struct {
		v    string
		want int64
		ok   bool
	}{
		{"1", 1, true},
		{"9223372036854775807", 9223372036854775807, true},
		{"", 0, false},
		{"0", 0, false},
		{"-1", 0, false},
		{"2/extra", 0, false},
		{"../2", 0, false},
		{"abc", 0, false},
		{"1.5", 0, false},
		{" 1", 0, false},
		{"+2", 0, false},
		{"-0", 0, false},
		{"2 ", 0, false},
		{"٣", 0, false},
		{"007", 7, true},
		{"9223372036854775808", 0, false},
	}
	for _, c := range casos {
		id, err := httpx.ParseID(c.v)
		if !c.ok {
			assert.ErrorIs(t, err, httpx.ErrIDInvalido, "v=%q", c.v)
			continue
		}
		require.NoError(t, err, "v=%q", c.v)
		assert.Equal(t, c.want, id)
	}
}

func TestIDDoPath_PeloMux(t *testing.T) {
	mux := http.NewServeMux()
	var id int64
	var err error
	mux.HandleFunc("GET /x/{id}", func(w http.ResponseWriter, r *http.Request) { id, err = httpx.IDDoPath(r, "id") })

	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x/42", nil))
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)

	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x/0", nil))
	assert.ErrorIs(t, err, httpx.ErrIDInvalido)

	w := httptest.NewRecorder()
	httpx.ResponderErro(w, "[x] Get", err)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.JSONEq(t, `{"success":false,"error":"id inválido"}`, w.Body.String())
}
