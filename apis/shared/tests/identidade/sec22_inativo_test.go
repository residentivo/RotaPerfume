package identidade_test

// SEC-22 (Lote 24, TestBrain): login e refresh não revelam conta inativa.
//   Login (credencial):
//   - inexistente: hash descartável (1) e nenhuma verificação real;
//   - inativo (senha errada ou certa): verificação real SEMPRE (1), sem
//     descartável; devolve ErrCredenciaisInvalidas mesmo com a senha certa;
//   - ativo com senha errada: verificação real (1).
//   Em todos: o MESMO erro (assert.Equal no sentinela), falha registrada em
//   "ip:" e "acct:", e nada da abertura de sessão (Emitir, RegistrarLogin,
//   RegravarHash, refresh). Inativo com bcrypt legado e senha certa não migra.
//   Refresh de inativo: revoga o token (motivo inativacao, best-effort),
//   conta falha em "refresh-ip:" e responde ErrRefreshRevogado sem emitir
//   nada; falha na revogação não muda a resposta.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/kernel"
)

// emissorEspiao conta as emissões de JWT.
type emissorEspiao struct{ emitidos int }

func (e *emissorEspiao) Emitir(int64, kernel.Role) (string, error) {
	e.emitidos++
	return "jwt", nil
}

// usuariosEspiao conta os RegistrarLogin (o fake base não conta).
type usuariosEspiao struct {
	*usuariosFake
	logins int
}

func (u *usuariosEspiao) RegistrarLogin(context.Context, int64, time.Time) error {
	u.logins++
	return nil
}

// hasherRehashEspiao: o hasherEspiao com PrecisaRehash ligado e contagem de
// Gerar (um rehash indevido apareceria aqui).
type hasherRehashEspiao struct {
	*hasherEspiao
	gerados int
}

func (h *hasherRehashEspiao) PrecisaRehash(identidade.HashSenha) bool { return true }
func (h *hasherRehashEspiao) Gerar(s identidade.Senha) (identidade.HashSenha, error) {
	h.gerados++
	return h.hasherFake.Gerar(s)
}

func sec22Inativo(id int64, email, senha string) *identidade.Usuario {
	u := usuarioAtivo(id, email, senha, kernel.RoleNormal)
	u.Ativo = false
	return u
}

func TestSEC22_Login_InativoNaoSeDistingue(t *testing.T) {
	const ip = "203.0.113.9"
	casos := []struct {
		nome             string
		email, senha     string
		wantDescartavel  int
		wantVerificacoes int
	}{
		{"e-mail inexistente", "nao@existe.c", "Atual#123", 1, 0},
		{"inativo com senha errada", "inativo@b.c", "Errada#123", 0, 1},
		{"inativo com senha certa", "inativo@b.c", "Atual#123", 0, 1},
		{"ativo com senha errada", "ativo@b.c", "Errada#123", 0, 1},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			a := novoAmbiente(sec22Inativo(3, "inativo@b.c", "Atual#123"), usuarioAtivo(4, "ativo@b.c", "Atual#123", kernel.RoleNormal))
			h := &hasherRehashEspiao{hasherEspiao: &hasherEspiao{hasherFake: a.hasher}}
			e := &emissorEspiao{}
			us := &usuariosEspiao{usuariosFake: a.usuarios}
			l := &limitadorEspiao{}
			d := a.deps()
			d.Hasher, d.Emissor, d.Usuarios = h, e, us
			d.Limites = identidadeapp.Limites{Login: l, Refresh: l, Reset: l}

			s, err := identidadeapp.Novo(d, identidadeapp.Opcoes{}).Login(ctx, identidadeapp.ComandoLogin{
				Email: c.email, Senha: identidade.NovaSenha(c.senha), Captcha: "ok", Origem: identidadeapp.Origem{IP: ip}})

			assert.Nil(t, s)
			assert.Equal(t, identidade.ErrCredenciaisInvalidas, err, "mesmo erro, sem embrulho, nos quatro casos")
			assert.Equal(t, c.wantDescartavel, h.descartaveis, "hash descartável")
			assert.Equal(t, c.wantVerificacoes, h.verificacoes, "verificação real")
			assert.Contains(t, l.chamadas, "falha:ip:"+ip)
			assert.Contains(t, l.chamadas, "falha:acct:"+ip+"|"+c.email)
			assert.NotContains(t, l.chamadas, "sucesso:ip:"+ip)
			assert.NotContains(t, l.chamadas, "sucesso:acct:"+ip+"|"+c.email)
			assert.Zero(t, e.emitidos, "nenhum JWT")
			assert.Zero(t, us.logins, "nenhum RegistrarLogin")
			assert.Zero(t, a.usuarios.rehash, "nenhum RegravarHash")
			assert.Zero(t, h.gerados, "nenhum rehash gerado")
			assert.Zero(t, a.tokens.inseridos, "nenhum refresh token")
		})
	}
}

// Sanidade dos espiões: o ativo com senha certa abre sessão (Emitir,
// RegistrarLogin, rehash e refresh), registra sucesso e não conta falha.
func TestSEC22_Login_AtivoSenhaCerta_AbreSessao(t *testing.T) {
	const ip = "203.0.113.9"
	a := novoAmbiente(usuarioAtivo(4, "ativo@b.c", "Atual#123", kernel.RoleNormal))
	h := &hasherRehashEspiao{hasherEspiao: &hasherEspiao{hasherFake: a.hasher}}
	e := &emissorEspiao{}
	us := &usuariosEspiao{usuariosFake: a.usuarios}
	l := &limitadorEspiao{}
	d := a.deps()
	d.Hasher, d.Emissor, d.Usuarios = h, e, us
	d.Limites = identidadeapp.Limites{Login: l, Refresh: l, Reset: l}

	s, err := identidadeapp.Novo(d, identidadeapp.Opcoes{}).Login(ctx, identidadeapp.ComandoLogin{
		Email: "ativo@b.c", Senha: identidade.NovaSenha("Atual#123"), Captcha: "ok", Origem: identidadeapp.Origem{IP: ip}})
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, 1, h.verificacoes)
	assert.Equal(t, 1, e.emitidos)
	assert.Equal(t, 1, us.logins)
	assert.Equal(t, 1, a.usuarios.rehash)
	assert.Equal(t, 1, a.tokens.inseridos)
	assert.Contains(t, l.chamadas, "sucesso:ip:"+ip)
	assert.NotContains(t, l.chamadas, "falha:ip:"+ip)
}

// Inativo com hash bcrypt legado e senha certa: o hasher real verifica (e
// diz que precisa de rehash), mas o login é recusado e o hash não migra.
// O ativo com o mesmo hash migra (sanidade do cenário).
func TestSEC22_Login_InativoBcryptLegadoNaoMigra(t *testing.T) {
	legado, err := bcrypt.GenerateFromPassword([]byte("Atual#123"), bcrypt.MinCost)
	require.NoError(t, err)
	real := identidadecrypto.NovoHasher(config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1})
	hash := identidade.NovoHashSenha(string(legado))
	require.True(t, real.Verificar(hash, identidade.NovaSenha("Atual#123")), "pré-condição: bcrypt legado confere")
	require.True(t, real.PrecisaRehash(hash), "pré-condição: bcrypt legado precisa de rehash")

	for _, ativo := range []bool{false, true} {
		t.Run(map[bool]string{false: "inativo não migra", true: "ativo migra (sanidade)"}[ativo], func(t *testing.T) {
			u := &identidade.Usuario{ID: 5, Nome: "L", Email: "legado@b.c", Role: kernel.RoleNormal, Ativo: ativo, Hash: hash}
			a := novoAmbiente(u)
			e := &emissorEspiao{}
			d := a.deps()
			d.Hasher, d.Emissor = real, e

			s, err := identidadeapp.Novo(d, identidadeapp.Opcoes{}).Login(ctx, identidadeapp.ComandoLogin{
				Email: "legado@b.c", Senha: identidade.NovaSenha("Atual#123"), Captcha: "ok", Origem: identidadeapp.Origem{IP: "1.2.3.4"}})
			if ativo {
				require.NoError(t, err)
				require.NotNil(t, s)
				assert.Equal(t, 1, a.usuarios.rehash)
				return
			}
			assert.Nil(t, s)
			assert.Equal(t, identidade.ErrCredenciaisInvalidas, err)
			assert.Zero(t, a.usuarios.rehash, "inativo nunca migra o hash")
			assert.Zero(t, e.emitidos)
			assert.Zero(t, a.tokens.inseridos)
		})
	}
}

// 5 falhas de inativo com senha certa bloqueiam a conta/IP como qualquer
// senha errada (limitador real).
func TestSEC22_Login_InativoContaNoRateLimit(t *testing.T) {
	a := novoAmbiente(sec22Inativo(3, "inativo@b.c", "Atual#123"))
	d := a.deps()
	l := &limitadorContador{limite: 5}
	d.Limites = identidadeapp.Limites{Login: l, Refresh: l, Reset: l}
	c := identidadeapp.Novo(d, identidadeapp.Opcoes{})
	cmd := identidadeapp.ComandoLogin{Email: "inativo@b.c", Senha: identidade.NovaSenha("Atual#123"), Captcha: "ok", Origem: identidadeapp.Origem{IP: "1.2.3.4"}}
	for i := 0; i < 5; i++ {
		_, err := c.Login(ctx, cmd)
		require.Equal(t, identidade.ErrCredenciaisInvalidas, err, "tentativa %d", i+1)
	}
	_, err := c.Login(ctx, cmd)
	assert.ErrorIs(t, err, identidade.ErrMuitasTentativas, "6ª tentativa bloqueada")
	assert.Equal(t, 5, l.falhas["ip:1.2.3.4"])
	assert.Equal(t, 5, l.falhas["acct:1.2.3.4|inativo@b.c"])
}

// limitadorContador bloqueia a chave após `limite` falhas.
type limitadorContador struct {
	limite int
	falhas map[string]int
}

func (l *limitadorContador) Bloqueado(chave string) (bool, time.Duration) {
	return l.falhas[chave] >= l.limite, time.Minute
}
func (l *limitadorContador) RegistrarFalha(chave string) {
	if l.falhas == nil {
		l.falhas = map[string]int{}
	}
	l.falhas[chave]++
}
func (l *limitadorContador) RegistrarSucesso(chave string) { delete(l.falhas, chave) }

func TestSEC22_Refresh_Inativo(t *testing.T) {
	const ip = "198.51.100.7"
	casos := []struct {
		nome       string
		errRevogar error
	}{
		{"revogação ok", nil},
		{"revogação falha (best-effort)", errInfra},
		{"token já revogado na corrida", identidade.ErrRefreshRevogado},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			a := novoAmbiente(sec22Inativo(5, "inativo@b.c", "Atual#123"))
			a.tokens.porHash["hash:tok"] = &identidade.RefreshToken{ID: 9, UsuarioID: 5, HashToken: "hash:tok", ExpiraEm: a.agora.Add(time.Hour)}
			a.tokens.errRevogar = c.errRevogar
			e := &emissorEspiao{}
			l := &limitadorEspiao{}
			d := a.deps()
			d.Emissor = e
			d.Limites = identidadeapp.Limites{Login: l, Refresh: l, Reset: l}
			casos := identidadeapp.Novo(d, identidadeapp.Opcoes{})

			for chamada := 1; chamada <= 2; chamada++ {
				s, err := casos.RenovarSessao(ctx, identidade.NovoTokenOpaco("tok"), identidadeapp.Origem{IP: ip})
				assert.Nil(t, s, "chamada %d", chamada)
				assert.Equal(t, identidade.ErrRefreshRevogado, err, "chamada %d: mesma resposta de token revogado", chamada)
			}
			if c.errRevogar == nil {
				assert.Equal(t, []identidade.MotivoRevogacao{identidade.MotivoInativacao, identidade.MotivoInativacao}, a.tokens.revogados)
			} else {
				assert.Empty(t, a.tokens.revogados)
			}
			falhas := 0
			for _, ch := range l.chamadas {
				if ch == "falha:refresh-ip:"+ip {
					falhas++
				}
				assert.NotEqual(t, "sucesso:refresh-ip:"+ip, ch)
			}
			assert.Equal(t, 2, falhas, "cada refresh de inativo conta no rate limit por IP")
			assert.Zero(t, e.emitidos, "nenhum JWT")
			assert.Zero(t, a.tokens.inseridos, "nenhum refresh novo")
		})
	}
}
