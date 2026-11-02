package identidade_test

// BUG-23 (Lote 31): refresh de usuário apagado segue o caminho do inativo
// (SEC-22): revoga o token (motivo inativacao), conta falha no rate limit
// por IP e responde ErrRefreshRevogado (401 genérico), sem emitir nada. Outro
// erro na releitura continua 500.

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
)

func ambienteBUG23(t *testing.T) (*ambiente, *limitadorEspiao, *emissorEspiao, *identidadeapp.Casos) {
	t.Helper()
	a := novoAmbiente() // nenhum usuário: o dono do token foi apagado
	a.tokens.porHash["hash:tok"] = &identidade.RefreshToken{ID: 21, UsuarioID: 77, HashToken: "hash:tok", ExpiraEm: a.agora.Add(time.Hour)}
	l := &limitadorEspiao{}
	e := &emissorEspiao{}
	d := a.deps()
	d.Emissor = e
	d.Limites = identidadeapp.Limites{Login: l, Refresh: l, Reset: l}
	return a, l, e, identidadeapp.Novo(d, identidadeapp.Opcoes{})
}

func TestBUG23_Refresh_UsuarioApagado_RevogaE401(t *testing.T) {
	const ip = "203.0.113.9"
	buf := capturarLogSEC32(t)
	a, l, e, casos := ambienteBUG23(t)

	s, err := casos.RenovarSessao(ctx, identidade.NovoTokenOpaco("tok"), identidadeapp.Origem{IP: ip})

	assert.Nil(t, s)
	assert.Equal(t, identidade.ErrRefreshRevogado, err, "mesma resposta do inativo (401 genérico)")
	assert.Equal(t, []identidade.MotivoRevogacao{identidade.MotivoInativacao}, a.tokens.revogados, "token revogado com motivo inativacao")
	assert.Contains(t, l.chamadas, "falha:refresh-ip:"+ip, "conta no rate limit por IP")
	assert.NotContains(t, l.chamadas, "sucesso:refresh-ip:"+ip)
	assert.Zero(t, e.emitidos, "nenhum JWT")
	assert.Zero(t, a.tokens.inseridos, "nenhum refresh novo")
	assert.Contains(t, buf.String(), "usuário inexistente user_id=77")
}

func TestBUG23_Refresh_UsuarioApagado_RevogacaoFalhaNaoMudaResposta(t *testing.T) {
	a, _, _, casos := ambienteBUG23(t)
	a.tokens.errRevogar = errInfra

	_, err := casos.RenovarSessao(ctx, identidade.NovoTokenOpaco("tok"), identidadeapp.Origem{IP: "203.0.113.9"})

	assert.Equal(t, identidade.ErrRefreshRevogado, err)
}

func TestBUG23_Refresh_ErroDeInfraNaReleituraContinua500(t *testing.T) {
	a, _, _, casos := ambienteBUG23(t)
	a.usuarios.errPorID = []error{errInfra}

	_, err := casos.RenovarSessao(ctx, identidade.NovoTokenOpaco("tok"), identidadeapp.Origem{IP: "203.0.113.9"})

	require.Error(t, err)
	assert.False(t, errors.Is(err, identidade.ErrRefreshRevogado), "infra não vira 401")
	assert.False(t, errors.Is(err, identidade.ErrUsuarioNaoEncontrado), "sem %%w: não vira 404")
	assert.Empty(t, a.tokens.revogados, "infra não revoga o token")
}
