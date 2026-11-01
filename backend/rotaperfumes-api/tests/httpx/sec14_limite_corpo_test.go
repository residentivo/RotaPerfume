package httpx_test

// SEC-14 (Lote 27): teto do corpo por rota no Router (256 KiB padrão;
// Rota.MaxCorpo próprio só no upload), com a ordem Exigir → limitarCorpo →
// Trace: 401/403 antes do 413. KindMuitoGrande → 413 tipado.

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
)

const sec14Msg413 = `{"success":false,"error":"corpo da requisição excede o limite de 256 KB"}`

// semTamanho esconde o tamanho do reader: httptest.NewRequest não consegue
// preencher o Content-Length (corpo "chunked", ContentLength = -1).
type semTamanho struct{ io.Reader }

// sec14Rota registra uma rota POST que decodifica o corpo com
// DecodificarJSON e conta as chamadas do handler.
func sec14Rota(t *testing.T, policy httpx.Policy, maxCorpo int64, chamadas *int) http.Handler {
	t.Helper()
	r := novoRouter(checkerFake{})
	r.Handle(httpx.Rota{Metodo: http.MethodPost, Padrao: "/api/x", Policy: policy, MaxCorpo: maxCorpo,
		Arquivo: "x_handler.go", Funcao: "XHandler.Post",
		Handler: func(w http.ResponseWriter, req *http.Request) {
			*chamadas++
			var dto struct {
				S string `json:"s"`
			}
			if err := httpx.DecodificarJSON(req, &dto); err != nil {
				httpx.ResponderErro(w, "[x] Post", err)
				return
			}
			httpx.JSON(w, http.StatusOK, len(dto.S), "")
		}})
	return r.Mux()
}

// corpoJSON devolve {"s":"aaa..."} com exatamente n bytes.
func corpoJSON(n int) []byte {
	const moldura = len(`{"s":""}`)
	return []byte(`{"s":"` + strings.Repeat("a", n-moldura) + `"}`)
}

func sec14Post(t *testing.T, h http.Handler, body io.Reader, contentLength int64, tok string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/x", body)
	req.ContentLength = contentLength
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestSEC14_Router_CorpoExatamenteNoLimiteAceito(t *testing.T) {
	chamadas := 0
	h := sec14Rota(t, httpx.Authenticated, 0, &chamadas)
	corpo := corpoJSON(int(httpx.MaxCorpoPadrao))
	require.Len(t, corpo, 256<<10)

	w := sec14Post(t, h, bytes.NewReader(corpo), int64(len(corpo)), token(t, 2, "normal"))
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = sec14Post(t, h, semTamanho{bytes.NewReader(corpo)}, -1, token(t, 2, "normal"))
	assert.Equal(t, http.StatusOK, w.Code, "chunked de exatamente 256 KiB")
	assert.Equal(t, 2, chamadas)
}

func TestSEC14_Router_ChunkedAcimaDoLimite_413(t *testing.T) {
	chamadas := 0
	h := sec14Rota(t, httpx.Authenticated, 0, &chamadas)
	corpo := corpoJSON(int(httpx.MaxCorpoPadrao) + 1)

	w := sec14Post(t, h, semTamanho{bytes.NewReader(corpo)}, -1, token(t, 2, "normal"))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.JSONEq(t, sec14Msg413, w.Body.String())
	assert.Equal(t, 1, chamadas, "chunked: o handler lê e o MaxBytesReader corta")
}

func TestSEC14_Router_ContentLengthAcimaDoLimite_413SemChamarHandler(t *testing.T) {
	chamadas := 0
	h := sec14Rota(t, httpx.Authenticated, 0, &chamadas)
	conta := &contaLeitura{r: strings.NewReader(`{"s":"x"}`)}

	w := sec14Post(t, h, conta, httpx.MaxCorpoPadrao+1, token(t, 2, "normal"))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.JSONEq(t, sec14Msg413, w.Body.String())
	assert.Zero(t, chamadas, "Content-Length acima do teto: handler não é chamado")
	assert.Zero(t, conta.lido, "nem o corpo é lido")
}

func TestSEC14_Router_AutenticacaoAntesDo413(t *testing.T) {
	chamadas := 0
	h := sec14Rota(t, httpx.AdminOnly, 0, &chamadas)
	grande := corpoJSON(int(httpx.MaxCorpoPadrao) + 10)

	w := sec14Post(t, h, bytes.NewReader(grande), int64(len(grande)), "")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "sem token: 401, não 413")

	w = sec14Post(t, h, bytes.NewReader(grande), int64(len(grande)), token(t, 2, "normal"))
	assert.Equal(t, http.StatusForbidden, w.Code, "vendedor em rota admin: 403, não 413")

	w = sec14Post(t, h, bytes.NewReader(grande), int64(len(grande)), token(t, 1, "admin"))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Zero(t, chamadas)
}

func TestSEC14_Router_RotaPublicaTambemLimitada(t *testing.T) {
	chamadas := 0
	h := sec14Rota(t, httpx.Public, 0, &chamadas)
	grande := corpoJSON(int(httpx.MaxCorpoPadrao) + 1)
	w := sec14Post(t, h, semTamanho{bytes.NewReader(grande)}, -1, "")
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

// Teto próprio (upload): o router só aplica o MaxBytesReader com o teto da
// rota; a checagem do Content-Length e a mensagem são do handler.
func TestSEC14_Router_TetoProprio(t *testing.T) {
	const teto = 512 << 10
	chamadas := 0
	h := sec14Rota(t, httpx.Authenticated, teto, &chamadas)

	acimaDoPadrao := corpoJSON(int(httpx.MaxCorpoPadrao) + 1)
	w := sec14Post(t, h, bytes.NewReader(acimaDoPadrao), int64(len(acimaDoPadrao)), token(t, 2, "normal"))
	assert.Equal(t, http.StatusOK, w.Code, "acima do padrão, mas dentro do teto da rota")

	acimaDoTeto := corpoJSON(teto + 1)
	w = sec14Post(t, h, semTamanho{bytes.NewReader(acimaDoTeto)}, -1, token(t, 2, "normal"))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code, "o MaxBytesReader do router corta no teto da rota")
	assert.Equal(t, 2, chamadas)
}

func TestSEC14_Router_MaxCorpoNegativoPanica(t *testing.T) {
	rt := rotaValida()
	rt.MaxCorpo = -1
	r := novoRouter(checkerFake{})
	assert.Panics(t, func() { r.Handle(rt) })
	assert.Empty(t, r.Rotas())
}

func TestSEC14_Router_LogDeDecisaoSemConteudo(t *testing.T) {
	out := capturarLog(t, true)
	chamadas := 0
	h := sec14Rota(t, httpx.Authenticated, 0, &chamadas)
	conta := &contaLeitura{r: strings.NewReader(`{"s":"SEGREDO-NO-CORPO"}`)}
	sec14Post(t, h, conta, httpx.MaxCorpoPadrao+1, token(t, 2, "normal"))
	assert.Contains(t, out.String(), "[x_handler.go] [XHandler.Post] decisão corpo_muito_grande limite=262144")
	assert.NotContains(t, out.String(), "SEGREDO")
}

func TestSEC14_LerAtivoOpcional_CorpoGrande_413(t *testing.T) {
	r := novoRouter(checkerFake{})
	chamadas := 0
	r.Handle(httpx.Rota{Metodo: http.MethodPatch, Padrao: "/api/x/{id}/inativar", Policy: httpx.Authenticated,
		Arquivo: "x_handler.go", Funcao: "XHandler.Toggle",
		Handler: func(w http.ResponseWriter, req *http.Request) {
			chamadas++
			if _, err := httpx.LerAtivoOpcional(req); err != nil {
				httpx.ResponderErro(w, "[x] Toggle", err)
				return
			}
			w.WriteHeader(http.StatusOK)
		}})
	// {"ativo":true} seguido de espaços: só o Token() depois do objeto estoura.
	corpo := `{"ativo":true}` + strings.Repeat(" ", int(httpx.MaxCorpoPadrao))
	req := httptest.NewRequest(http.MethodPatch, "/api/x/1/inativar", semTamanho{strings.NewReader(corpo)})
	req.ContentLength = -1
	req.Header.Set("Authorization", "Bearer "+token(t, 2, "normal"))
	w := httptest.NewRecorder()
	r.Mux().ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.JSONEq(t, sec14Msg413, w.Body.String())
	assert.Equal(t, 1, chamadas)
}

func TestSEC14_Body_MaxBytesErrorVira413_Resto400(t *testing.T) {
	limitado := func(body string, n int64) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, n)
		return req
	}
	var dst map[string]any
	assert.ErrorIs(t, httpx.DecodificarJSON(limitado(`{"a":"0123456789"}`, 5), &dst), httpx.ErrCorpoMuitoGrande)
	assert.ErrorIs(t, httpx.DecodificarJSON(limitado(`{invalido`, 100), &dst), httpx.ErrBodyJSONInvalido)

	_, err := httpx.LerAtivoOpcional(limitado(`{"ativo":true,"x":"0123456789"}`, 5))
	assert.ErrorIs(t, err, httpx.ErrCorpoMuitoGrande)
	_, err = httpx.LerAtivoOpcional(limitado(`{"ativo":1}`, 100))
	assert.ErrorIs(t, err, httpx.ErrBodyJSONInvalido)

	w := httptest.NewRecorder()
	httpx.ResponderErro(w, "[teste]", httpx.ErrCorpoMuitoGrande)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.JSONEq(t, sec14Msg413, w.Body.String())
}

// contaLeitura conta os bytes lidos do corpo.
type contaLeitura struct {
	r    io.Reader
	lido int
}

func (c *contaLeitura) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.lido += n
	return n, err
}
