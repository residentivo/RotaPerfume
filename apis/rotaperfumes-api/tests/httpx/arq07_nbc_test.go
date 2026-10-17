package httpx_test

// ARQ-07 NB-C: r.URL.Path chega decodificado (%0A vira "\n"); nenhum log
// com o path (acesso, recusa do Autenticador, rota no contexto) pode ganhar
// uma linha nova forjada.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/kernel"
)

const alvoInjecao = "/api/x%0A[escopo]%20forjado%0D"

func TestARQ07_NBC_PathComQuebraNaoGeraLinhaNoLog(t *testing.T) {
	logs := capturarLog(t, false)
	inativo := checkerFake{status: map[int64]identidade.StatusAcesso{2: {Ativo: false, Role: "normal"}}}
	a := httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), inativo)
	h := httpx.Logging(a.Exigir(httpx.Authenticated, http.HandlerFunc(handlerOK)))

	w := servir(h, "GET", alvoInjecao, token(t, 2, "normal"))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	saida := logs.String()
	assert.Contains(t, saida, `[auth] acesso negado: user_id=2 inativo GET /api/x\n[escopo] forjado\r`)
	assert.Contains(t, saida, `[http] GET /api/x\n[escopo] forjado\r -> 401`)
	for _, linha := range strings.Split(strings.TrimRight(saida, "\n"), "\n") {
		assert.False(t, strings.HasPrefix(linha, "[escopo]"), "linha forjada no log: %q", linha)
	}
	assert.NotContains(t, saida, "\r")
}

func TestARQ07_NBC_RotaNoContextoSaneada(t *testing.T) {
	var rota string
	a := httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), checkerFake{})
	h := a.Exigir(httpx.Authenticated, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rota = kernel.RotaDe(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := servir(h, "GET", alvoInjecao, token(t, 2, "normal"))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `/api/x\n[escopo] forjado\r`, rota)
}
