package httpx_test

// ARQ-05 (cobertura do portão): o statusRecorder do httpx.Logging expõe o
// writer original por Unwrap, para o http.ResponseController (Flush etc.)
// funcionar atrás do log de acesso.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
)

func TestARQ05_Logging_ResponseControllerAlcancaOWriter(t *testing.T) {
	var errFlush error
	h := httpx.Logging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		errFlush = http.NewResponseController(w).Flush()
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.NoError(t, errFlush, "Flush chega ao ResponseRecorder via Unwrap")
	assert.True(t, rec.Flushed)
	assert.Equal(t, http.StatusAccepted, rec.Code)
}
