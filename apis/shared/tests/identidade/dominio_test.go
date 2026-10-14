package identidade_test

// ARQ-03: domínio de Identidade (puro): VOs de segredo (I-12), política de
// senha, validação de usuário, corte de sessão (I-04), RefreshToken.Avaliar
// (I-06/I-07) e o bloqueio de reuso sem short-circuit (I-12).

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

func TestSegredos_NuncaVazamPorPrintNemJSON(t *testing.T) {
	segredos := []any{
		identidade.NovaSenha("Senha#Secreta1"),
		identidade.NovoHashSenha("$argon2id$v=19$segredo"),
		identidade.NovoTokenOpaco("token-opaco-secreto"),
	}
	for _, s := range segredos {
		t.Run(fmt.Sprintf("%T", s), func(t *testing.T) {
			for _, formato := range []string{"%v", "%+v", "%#v", "%s"} {
				assert.Equal(t, "***", fmt.Sprintf(formato, s))
			}
			_, err := json.Marshal(s)
			assert.ErrorIs(t, err, identidade.ErrSerializarSegredo)
			_, err = json.Marshal(map[string]any{"campo": s})
			assert.ErrorIs(t, err, identidade.ErrSerializarSegredo, "aninhado também falha")
			m, ok := s.(interface{ MarshalText() ([]byte, error) })
			require.True(t, ok)
			_, err = m.MarshalText()
			assert.ErrorIs(t, err, identidade.ErrSerializarSegredo)
		})
	}
	// Dentro de uma entidade, o campo de hash continua mascarado.
	u := identidade.Usuario{ID: 1, Hash: identidade.NovoHashSenha("$argon2id$segredo")}
	assert.NotContains(t, fmt.Sprintf("%+v", u), "segredo")
}

func TestSegredos_RevelarEComparar(t *testing.T) {
	s := identidade.NovaSenha("abc")
	assert.Equal(t, "abc", s.Revelar())
	assert.False(t, s.Vazia())
	assert.True(t, identidade.NovaSenha("").Vazia())
	assert.True(t, s.Igual(identidade.NovaSenha("abc")))
	assert.False(t, s.Igual(identidade.NovaSenha("abd")))
	assert.Equal(t, "h", identidade.NovoHashSenha("h").Revelar())
	tok := identidade.NovoTokenOpaco("t")
	assert.Equal(t, "t", tok.Revelar())
	assert.False(t, tok.Vazio())
	assert.True(t, identidade.NovoTokenOpaco("").Vazio())
}

func TestValidarForca(t *testing.T) {
	casos := []struct {
		senha string
		want  error
	}{
		{"Abcdef1!", nil},
		{"abcdefg1", identidade.ErrSenhaFraca},
		{"Ab1!", identidade.ErrSenhaCurta},
		{"áéíóúçãõ", identidade.ErrSenhaFraca}, // 8 runes, uma classe
		{string(make([]byte, 73)), identidade.ErrSenhaMuitoLonga},
		{"ABCDEF1-", nil}, // maiúscula, dígito, símbolo
	}
	for _, c := range casos {
		assert.Equal(t, c.want, identidade.ValidarForca(identidade.NovaSenha(c.senha)), c.senha)
	}
}

func TestValidarCriacaoEEdicao(t *testing.T) {
	v, err := identidade.ValidarCriacao(identidade.DadosUsuario{Nome: " Ana ", Email: " Ana@RP.com ", Role: " admin "})
	require.NoError(t, err)
	assert.Equal(t, "Ana", v.Nome)
	assert.Equal(t, "ana@rp.com", v.Email)
	assert.Equal(t, kernel.RoleAdmin, v.Role)

	casos := []struct {
		d    identidade.DadosUsuario
		want error
	}{
		{identidade.DadosUsuario{Nome: " ", Email: "a@b.c", Role: "admin"}, identidade.ErrNomeObrigatorio},
		{identidade.DadosUsuario{Nome: "A", Email: " ", Role: "admin"}, identidade.ErrEmailObrigatorio},
		{identidade.DadosUsuario{Nome: "A", Email: "a@b.c", Role: "gerente"}, identidade.ErrRoleInvalido},
		{identidade.DadosUsuario{Nome: "", Email: "", Role: ""}, identidade.ErrNomeObrigatorio},
	}
	for _, c := range casos {
		_, err := identidade.ValidarCriacao(c.d)
		assert.Equal(t, c.want, err)
	}
	_, err = identidade.ValidarEdicao(identidade.DadosUsuario{Nome: "A", Role: "normal"})
	assert.NoError(t, err, "e-mail não é editável")
	_, err = identidade.ValidarEdicao(identidade.DadosUsuario{Nome: "A", Role: "x"})
	assert.Equal(t, identidade.ErrRoleInvalido, err)

	u := identidade.NovoUsuario(v, identidade.NovoHashSenha("h"))
	assert.True(t, u.Ativo)
	assert.True(t, u.DeveTrocarSenha, "senha gerada pelo sistema: troca obrigatória")
	assert.True(t, u.EhAdmin())
}

func TestUsuario_VendedorVinculado(t *testing.T) {
	zero, sete := int64(0), int64(7)
	_, ok := (&identidade.Usuario{}).VendedorVinculado()
	assert.False(t, ok)
	_, ok = (&identidade.Usuario{IDVendedor: &zero}).VendedorVinculado()
	assert.False(t, ok, "vínculo órfão")
	id, ok := (&identidade.Usuario{IDVendedor: &sete}).VendedorVinculado()
	assert.True(t, ok)
	assert.Equal(t, int64(7), id)
}

func TestStatusAcesso_TokenAnteriorAoCorte(t *testing.T) {
	corte := time.Date(2026, 9, 26, 10, 0, 0, 900_000_000, time.Local)
	antes, mesmo, depois := corte.Add(-time.Second), corte.Truncate(time.Second), corte.Add(time.Second)
	sem := identidade.StatusAcesso{}
	com := identidade.StatusAcesso{TokensValidosDesde: &corte}
	assert.False(t, sem.TokenAnteriorAoCorte(nil), "sem corte, vale")
	assert.True(t, com.TokenAnteriorAoCorte(nil), "com corte, token sem iat é recusado")
	assert.True(t, com.TokenAnteriorAoCorte(&antes))
	assert.True(t, com.TokenAnteriorAoCorte(&mesmo), "mesmo segundo é recusado")
	assert.False(t, com.TokenAnteriorAoCorte(&depois))
	assert.Equal(t, corte.Truncate(time.Second), identidade.CorteDeSessao(corte))
}

func TestRefreshToken_Avaliar(t *testing.T) {
	agora := time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)
	em := func(d time.Duration) *time.Time { t := agora.Add(d); return &t }
	casos := []struct {
		nome    string
		expira  time.Time
		revoked *time.Time
		motivo  identidade.MotivoRevogacao
		want    identidade.EstadoRefresh
		reuso   bool
	}{
		{"válido", agora.Add(time.Hour), nil, "", identidade.RefreshValido, false},
		{"expirado antes da revogação", agora.Add(-time.Second), em(-5 * time.Second), "rotacao", identidade.RefreshExpirado, false},
		{"revogado há 30s é recente", agora.Add(time.Hour), em(-30 * time.Second), "rotacao", identidade.RefreshRevogadoRecente, false},
		{"revogado 30s no futuro é recente", agora.Add(time.Hour), em(30 * time.Second), "rotacao", identidade.RefreshRevogadoRecente, false},
		{"revogado há 31s por rotação é reuso", agora.Add(time.Hour), em(-31 * time.Second), "rotacao", identidade.RefreshRevogado, true},
		{"revogado por logout não é reuso", agora.Add(time.Hour), em(-time.Hour), "logout", identidade.RefreshRevogado, false},
		{"NULL legado não é reuso", agora.Add(time.Hour), em(-time.Hour), identidade.MotivoDesconhecido, identidade.RefreshRevogado, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rt := &identidade.RefreshToken{ExpiraEm: c.expira, RevogadoEm: c.revoked, Motivo: c.motivo}
			e := rt.Avaliar(agora)
			assert.Equal(t, c.want, e)
			assert.Equal(t, c.reuso, rt.ReusoDeRotacao(e))
		})
	}
	novo := identidade.NovoRefreshToken(3, "h", agora, "ip", "ua")
	assert.Equal(t, agora.Add(identidade.TTLRefresh), novo.ExpiraEm)
	assert.Equal(t, identidade.RefreshValido, novo.Avaliar(agora))
}

// verificadorContador conta as verificações: prova que não há short-circuit.
type verificadorContador struct{ chamadas int }

func (v *verificadorContador) Verificar(h identidade.HashSenha, s identidade.Senha) bool {
	v.chamadas++
	return h.Revelar() == "hash:"+s.Revelar()
}

func TestSenhaReutilizada_SemShortCircuit(t *testing.T) {
	hashes := []identidade.HashSenha{identidade.NovoHashSenha("hash:atual"), identidade.NovoHashSenha("hash:velha1"), identidade.NovoHashSenha("hash:velha2")}
	for _, c := range []struct {
		senha string
		want  bool
	}{{"atual", true}, {"velha2", true}, {"nova", false}} {
		v := &verificadorContador{}
		assert.Equal(t, c.want, identidade.SenhaReutilizada(identidade.NovaSenha(c.senha), hashes, v))
		assert.Equal(t, 3, v.chamadas, "todas as entradas são avaliadas (tempo não revela qual casou)")
	}
}

func TestOrdenacoesPadrao(t *testing.T) {
	assert.Equal(t, identidade.OrdemUsuarioID, identidade.OrdenacaoUsuarioPadrao().Campo())
	assert.Equal(t, kernel.Asc, identidade.OrdenacaoUsuarioPadrao().Direcao())
	assert.Equal(t, identidade.OrdemHistoricoID, identidade.OrdenacaoHistoricoPadrao().Campo())
	assert.Equal(t, kernel.Desc, identidade.OrdenacaoHistoricoPadrao().Direcao())
	assert.Len(t, identidade.CamposOrdenacaoUsuario(), 8)
	assert.Len(t, identidade.CamposOrdenacaoHistorico(), 7)
}
