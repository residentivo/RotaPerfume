package httpx_test

// httpx.ResponderErro (F-05, O6): switch sobre Kind com default 500, mensagem
// do sentinela (nunca do wrap), 429 com Retry-After ≥ 1 s e tags de log.

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/kernel"
)

// TestErrMap_TodosOsKinds percorre Kind(0)..KindMuitoGrande+5: os conhecidos
// têm o status do enum; os de fora caem em 500 "erro interno" e logam.
func TestErrMap_TodosOsKinds(t *testing.T) {
	esperado := map[kernel.Kind]int{
		kernel.KindInterno:        http.StatusInternalServerError,
		kernel.KindInvalido:       http.StatusBadRequest,
		kernel.KindNaoEncontrado:  http.StatusNotFound,
		kernel.KindConflito:       http.StatusConflict,
		kernel.KindProibido:       http.StatusForbidden,
		kernel.KindNaoAutenticado: http.StatusUnauthorized,
		kernel.KindBloqueado:      http.StatusTooManyRequests,
		kernel.KindMuitoGrande:    http.StatusRequestEntityTooLarge,
	}
	for k := kernel.Kind(0); k <= kernel.KindMuitoGrande+5; k++ {
		t.Run(fmt.Sprintf("kind=%d(%s)", k, k), func(t *testing.T) {
			out := capturarLog(t, false)
			err := kernel.NovoErro(k, "mensagem do sentinela")
			w := httptest.NewRecorder()
			httpx.ResponderErro(w, "[teste] Handler", err)

			want, conhecido := esperado[k]
			if !conhecido {
				want = http.StatusInternalServerError
			}
			assert.Equal(t, want, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			if want == http.StatusInternalServerError {
				assert.JSONEq(t, `{"success":false,"error":"erro interno"}`, w.Body.String())
				assert.Contains(t, out.String(), "[teste] Handler", "500 sempre gera log")
				return
			}
			assert.JSONEq(t, `{"success":false,"error":"mensagem do sentinela"}`, w.Body.String())
		})
	}
}

var (
	errProdutoNaoEncontrado = kernel.NovoErro(kernel.KindNaoEncontrado, "produto não encontrado")
	errMuitasTentativas     = kernel.NovoErro(kernel.KindBloqueado, "muitas tentativas — tente novamente mais tarde")
)

func TestErrMap_MensagemDoSentinelaNuncaDoWrap(t *testing.T) {
	casos := []struct {
		nome string
		err  error
	}{
		{"wrap simples", fmt.Errorf("catalogomysql: obter produto: %w", errProdutoNaoEncontrado)},
		{"wrap duplo", fmt.Errorf("catalogoapp: %w", fmt.Errorf("catalogomysql: obter id=99: %w", errProdutoNaoEncontrado))},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpx.ResponderErro(w, "[produto] Get", c.err)
			assert.Equal(t, http.StatusNotFound, w.Code)
			assert.JSONEq(t, `{"success":false,"error":"produto não encontrado"}`, w.Body.String())
			assert.NotContains(t, w.Body.String(), "catalogomysql")
		})
	}
}

// R3/R-e: erro desconhecido, escopo e principal inválidos → 500 com log de tag fixa.
func TestErrMap_Interno_LogComTag(t *testing.T) {
	casos := []struct {
		nome    string
		err     error
		wantLog string
		semLog  string
	}{
		{"erro de infra", errors.New("driver: senha=xyz"), "[cliente] Create: driver: senha=xyz", ""},
		{"escopo inválido", fmt.Errorf("op: %w", kernel.ErrEscopoInvalido), "[escopo] invalido [cliente] Create", "op:"},
		{"escopo sem acesso", kernel.ErrEscopoSemAcesso, "[escopo] invalido [cliente] Create", ""},
		{"principal inválido", kernel.ErrPrincipalInvalido, "[auth] principal invalido [cliente] Create", ""},
		{"interno genérico", kernel.ErrInterno, "[cliente] Create: erro interno", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out := capturarLog(t, false)
			w := httptest.NewRecorder()
			httpx.ResponderErro(w, "[cliente] Create", c.err)
			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.JSONEq(t, `{"success":false,"error":"erro interno"}`, w.Body.String())
			assert.Contains(t, out.String(), c.wantLog)
			if c.semLog != "" {
				assert.NotContains(t, out.String(), c.semLog, "tag fixa, sem detalhes")
			}
			assert.NotContains(t, w.Body.String(), "senha")
		})
	}
}

// I-14: 429 com Retry-After ≥ 1 s (truncado, como o legado) e a mensagem do sentinela.
func TestErrMap_Bloqueado_RetryAfter(t *testing.T) {
	casos := []struct {
		nome  string
		err   error
		retry string
	}{
		{"42s", &kernel.Bloqueio{Sentinela: errMuitasTentativas, RetryAfter: 42 * time.Second}, "42"},
		{"truncado 1.9s", &kernel.Bloqueio{Sentinela: errMuitasTentativas, RetryAfter: 1900 * time.Millisecond}, "1"},
		{"menor que 1s vira 1", &kernel.Bloqueio{Sentinela: errMuitasTentativas, RetryAfter: 200 * time.Millisecond}, "1"},
		{"zero vira 1", &kernel.Bloqueio{Sentinela: errMuitasTentativas}, "1"},
		{"embrulhado", fmt.Errorf("identidadeapp: login: %w", &kernel.Bloqueio{Sentinela: errMuitasTentativas, RetryAfter: 5 * time.Minute}), "300"},
		{"sentinela sem Bloqueio", errMuitasTentativas, "1"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			httpx.ResponderErro(w, "[auth] Login", c.err)
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			assert.Equal(t, c.retry, w.Header().Get("Retry-After"))
			assert.JSONEq(t, `{"success":false,"error":"muitas tentativas — tente novamente mais tarde"}`, w.Body.String())
		})
	}
}

func TestResponderBloqueado_SemSentinela(t *testing.T) {
	w := httptest.NewRecorder()
	httpx.ResponderBloqueado(w, errors.New("x"))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "1", w.Header().Get("Retry-After"))
	assert.JSONEq(t, `{"success":false,"error":"erro interno"}`, w.Body.String())
}
