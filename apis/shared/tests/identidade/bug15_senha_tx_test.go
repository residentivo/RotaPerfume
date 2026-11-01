package identidade_test

// BUG-15 (Lote 27): troca de senha, reset administrativo e reset em massa
// gravam histórico, senha (com o corte de sessão) e revogação dos refresh
// tokens numa tx só. O hash (Argon2) é gerado fora da tx; o e-mail só sai
// depois do commit. Mudança intencional: falha no histórico ou na
// revogação, que antes respondia 200, passa a 500 sem trocar a senha.

import (
	"bytes"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

// bug15Falhas são as etapas da tx que podem falhar.
var bug15Falhas = []struct {
	nome   string
	falhar func(a *ambiente)
}{
	{"Registrar histórico", func(a *ambiente) { a.historico.errRegistro = errInfra }},
	{"AtualizarSenha", func(a *ambiente) { a.usuarios.errSenha = errInfra }},
	{"RevogarTodos", func(a *ambiente) { a.tokens.errTodos = errInfra }},
	{"Commit", func(a *ambiente) { a.txSenha.errCommit = errInfra }},
}

func bug15CapturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	prev := log.Writer()
	log.SetOutput(out)
	t.Cleanup(func() { log.SetOutput(prev) })
	return out
}

// bug15Erro500: erro interno, nunca um sentinela de domínio (4xx).
func bug15Erro500(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var de *kernel.Erro
	assert.False(t, errors.As(err, &de), "500, não um sentinela de domínio")
}

// bug15NadaGravado: rollback total (senha, histórico, revogação).
func bug15NadaGravado(t *testing.T, a *ambiente) {
	t.Helper()
	assert.Empty(t, a.usuarios.senhas, "senha não muda")
	assert.Zero(t, a.historico.registros, "sem histórico")
	assert.Empty(t, a.tokens.revogados, "sem revogação")
	assert.Equal(t, 1, a.txSenha.rollbacks)
	assert.Zero(t, a.txSenha.commits)
}

func bug15CmdTroca() identidadeapp.ComandoTrocarSenha {
	return identidadeapp.ComandoTrocarSenha{SenhaAtual: identidade.NovaSenha("Atual#123"), NovaSenha: identidade.NovaSenha("Nova#12345"), Captcha: "ok"}
}

func TestBUG15_TrocarSenha_FalhaEmCadaEtapa_500Rollback(t *testing.T) {
	for _, f := range bug15Falhas {
		t.Run(f.nome, func(t *testing.T) {
			out := bug15CapturarLog(t)
			a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
			f.falhar(a)

			err := a.casos().TrocarSenha(ctx, principal(2, kernel.RoleNormal), bug15CmdTroca())
			bug15Erro500(t, err)
			bug15NadaGravado(t, a)
			for _, sucesso := range []string{"[senha-historico] registrado", "[refresh] todos tokens revogados", "[auth] reset-password: user_id="} {
				assert.NotContains(t, out.String(), sucesso, "log de sucesso só depois do commit")
			}
		})
	}
}

func TestBUG15_TrocarSenha_Sucesso_UmaTxComTudo(t *testing.T) {
	out := bug15CapturarLog(t)
	u := usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal)
	a := novoAmbiente(u)

	require.NoError(t, a.casos().TrocarSenha(ctx, principal(2, kernel.RoleNormal), bug15CmdTroca()))
	assert.Equal(t, []string{"commit"}, a.linhaDoTempo, "uma tx, sem e-mail")
	require.Len(t, a.usuarios.senhas, 1)
	assert.Equal(t, "h:Nova#12345", a.usuarios.senhas[0].Revelar())
	require.NotNil(t, a.historico.ultimo)
	assert.Equal(t, identidade.TipoResetUsuario, a.historico.ultimo.Tipo)
	assert.Nil(t, a.historico.ultimo.ResetadoPorID)
	assert.Equal(t, u.Hash, a.historico.ultimo.HashAnterior)
	assert.Equal(t, []identidade.MotivoRevogacao{identidade.MotivoSenha}, a.tokens.revogados)
	assert.Contains(t, out.String(), "[senha-historico] registrado: usuario_id=2")
	assert.Contains(t, out.String(), "[refresh] todos tokens revogados: usuario_id=2")
}

// O Argon2 roda fora da tx: hash que falha não abre tx nenhuma.
func TestBUG15_TrocarSenha_HashFalhaSemAbrirTx(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
	a.hasher.errGerar = errInfra
	assert.ErrorIs(t, a.casos().TrocarSenha(ctx, principal(2, kernel.RoleNormal), bug15CmdTroca()), errInfra)
	assert.Zero(t, a.txSenha.chamadas)
}

func TestBUG15_AdminResetarSenha_FalhaEmCadaEtapa_500SemEmail(t *testing.T) {
	cmd := identidadeapp.ComandoAdminResetarSenha{UsuarioID: 5}
	for _, f := range bug15Falhas {
		t.Run(f.nome, func(t *testing.T) {
			a := novoAmbiente(usuarioAtivo(5, "a@b.c", "Velha#123", kernel.RoleNormal))
			f.falhar(a)

			enviado, err := a.casos().AdminResetarSenha(ctx, principal(1, kernel.RoleAdmin), cmd)
			assert.False(t, enviado)
			bug15Erro500(t, err)
			bug15NadaGravado(t, a)
			assert.Equal(t, []string{"rollback"}, a.linhaDoTempo, "nenhum e-mail")
		})
	}
}

// Usuário apagado entre a releitura e o UPDATE: 404, como no legado.
func TestBUG15_AdminResetarSenha_SumiuNoUpdate_404SemEmail(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(5, "a@b.c", "Velha#123", kernel.RoleNormal))
	a.usuarios.errSenha = identidade.ErrUsuarioNaoEncontrado

	_, err := a.casos().AdminResetarSenha(ctx, principal(1, kernel.RoleAdmin), identidadeapp.ComandoAdminResetarSenha{UsuarioID: 5})
	assert.Same(t, identidade.ErrUsuarioNaoEncontrado, err)
	assert.Equal(t, []string{"rollback"}, a.linhaDoTempo)
}

func TestBUG15_AdminResetarSenha_Sucesso_EmailDepoisDoCommit(t *testing.T) {
	alvo := usuarioAtivo(5, "a@b.c", "Velha#123", kernel.RoleNormal)
	a := novoAmbiente(alvo)
	o := identidadeapp.Origem{IP: "10.0.0.1", UserAgent: "ua"}

	enviado, err := a.casos().AdminResetarSenha(ctx, principal(1, kernel.RoleAdmin), identidadeapp.ComandoAdminResetarSenha{UsuarioID: 5, Origem: o})
	require.NoError(t, err)
	assert.True(t, enviado)
	assert.Equal(t, []string{"commit", "email"}, a.linhaDoTempo, "1 e-mail, depois do commit")
	require.Len(t, a.usuarios.senhas, 1)
	require.NotNil(t, a.historico.ultimo)
	assert.Equal(t, identidade.TipoResetAdmin, a.historico.ultimo.Tipo)
	require.NotNil(t, a.historico.ultimo.ResetadoPorID)
	assert.Equal(t, int64(1), *a.historico.ultimo.ResetadoPorID, "por = admin")
	assert.Equal(t, alvo.Hash, a.historico.ultimo.HashAnterior, "hash anterior = o relido")
	assert.Equal(t, "10.0.0.1", a.historico.ultimo.IP)
	assert.Equal(t, []identidade.MotivoRevogacao{identidade.MotivoSenha}, a.tokens.revogados)
}

// Reset em massa: a tx do usuário N falha → o lote para, N não recebe
// e-mail e os anteriores ficam (commit e e-mail cada um).
func TestBUG15_ResetMassa_FalhaDeTxNoUsuarioN_InterrompeSemEmail(t *testing.T) {
	u1 := usuarioAtivo(1, "u1@b.c", "S1#1234567", kernel.RoleNormal)
	u2 := usuarioAtivo(2, "u2@b.c", "S2#1234567", kernel.RoleNormal)
	u3 := usuarioAtivo(3, "u3@b.c", "S3#1234567", kernel.RoleNormal)
	a := novoAmbiente(u1, u2, u3)
	a.usuarios.ativos = []identidade.Usuario{*u1, *u2, *u3}
	a.txSenha.errCommit, a.txSenha.falharNa = errInfra, 2

	itens, err := a.casos().ResetarSenhasAtivos(ctx, false)
	require.Error(t, err)
	assert.NotErrorIs(t, err, identidadeapp.ErrEmailResetFalhou, "falha de tx não é falha de e-mail")
	assert.Contains(t, err.Error(), "usuario_id=2")
	require.Len(t, itens, 1)
	assert.Equal(t, identidadeapp.StatusResetado, itens[0].Status)
	assert.Equal(t, []string{"commit", "email", "rollback"}, a.linhaDoTempo, "u2 sem e-mail; u3 nem começa")
	assert.Len(t, a.usuarios.senhas, 1, "só a senha do u1 ficou")
	assert.Equal(t, 1, a.historico.registros)
}

// Falha de e-mail no reset em massa continua ErrEmailResetFalhou, com o
// reset do usuário já gravado (commit antes do e-mail).
func TestBUG15_ResetMassa_FalhaDeEmail_ResetJaGravado(t *testing.T) {
	u1 := usuarioAtivo(1, "u1@b.c", "S1#1234567", kernel.RoleNormal)
	a := novoAmbiente(u1)
	a.usuarios.ativos = []identidade.Usuario{*u1}
	a.email = emailFake{err: errInfra, linhaDoTempo: &a.linhaDoTempo}

	itens, err := a.casos().ResetarSenhasAtivos(ctx, false)
	assert.ErrorIs(t, err, identidadeapp.ErrEmailResetFalhou)
	require.Len(t, itens, 1)
	assert.Equal(t, identidadeapp.StatusFalhaEmail, itens[0].Status)
	assert.Equal(t, []string{"commit", "email"}, a.linhaDoTempo)
	assert.Len(t, a.usuarios.senhas, 1)
	require.NotNil(t, a.historico.ultimo)
	assert.Nil(t, a.historico.ultimo.ResetadoPorID)
	assert.Equal(t, identidadeapp.ResetMassaIPOrigem, a.historico.ultimo.IP)
}
