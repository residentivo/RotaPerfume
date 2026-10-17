package identidade_test

// ARQ-03: casos de uso de identidadeapp sobre fakes. Cobrem os ramos que os
// testes HTTP (sqlmock) não alcançam: falhas de infraestrutura, defesa em
// profundidade de admin (I-05), Principal inválido (O5) e as falhas
// best-effort que só logam.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

var ctx = context.Background()

func TestNovo_DependenciaNilPanica(t *testing.T) {
	base := novoAmbiente().deps()
	zerar := []func(d *identidadeapp.Deps){
		func(d *identidadeapp.Deps) { d.Usuarios = nil },
		func(d *identidadeapp.Deps) { d.Tokens = nil },
		func(d *identidadeapp.Deps) { d.Historico = nil },
		func(d *identidadeapp.Deps) { d.Rotacao = nil },
		func(d *identidadeapp.Deps) { d.Hasher = nil },
		func(d *identidadeapp.Deps) { d.Emissor = nil },
		func(d *identidadeapp.Deps) { d.Refresh = nil },
		func(d *identidadeapp.Deps) { d.Senhas = nil },
		func(d *identidadeapp.Deps) { d.Captcha = nil },
		func(d *identidadeapp.Deps) { d.Limites.Login = nil },
		func(d *identidadeapp.Deps) { d.Limites.Refresh = nil },
		func(d *identidadeapp.Deps) { d.Limites.Reset = nil },
		func(d *identidadeapp.Deps) { d.Publicador = nil },
		func(d *identidadeapp.Deps) { d.Email = nil },
		func(d *identidadeapp.Deps) { d.Vendedores = nil },
		func(d *identidadeapp.Deps) { d.Relogio = nil },
	}
	for i, z := range zerar {
		d := base
		z(&d)
		assert.Panics(t, func() { identidadeapp.Novo(d, identidadeapp.Opcoes{}) }, "dependência %d", i)
	}
}

func TestLogin_RamosDeFalha(t *testing.T) {
	cmd := func(senha string) identidadeapp.ComandoLogin {
		return identidadeapp.ComandoLogin{Email: "a@b.c", Senha: identidade.NovaSenha(senha), Captcha: "ok", Origem: identidadeapp.Origem{IP: "1.2.3.4"}}
	}
	t.Run("conta bloqueada (IP+e-mail) → 429 com retry", func(t *testing.T) {
		a := novoAmbiente()
		a.limite = limitadorFake{bloquear: "acct:"}
		_, err := a.casos().Login(ctx, cmd("x"))
		var b *kernel.Bloqueio
		require.ErrorAs(t, err, &b)
		assert.Equal(t, 42*time.Second, b.RetryAfter)
		assert.ErrorIs(t, err, identidade.ErrMuitasTentativas)
	})
	t.Run("erro de infra na busca → 500 (não 401)", func(t *testing.T) {
		a := novoAmbiente()
		a.usuarios.errEmail = errInfra
		_, err := a.casos().Login(ctx, cmd("x"))
		assert.ErrorIs(t, err, errInfra)
		assert.NotErrorIs(t, err, identidade.ErrCredenciaisInvalidas)
	})
	t.Run("falha ao emitir o JWT → ErrGerarToken", func(t *testing.T) {
		a := novoAmbiente(usuarioAtivo(1, "a@b.c", "s", kernel.RoleNormal))
		a.emissor = emissorFake{err: errInfra}
		_, err := a.casos().Login(ctx, cmd("s"))
		assert.Equal(t, identidade.ErrGerarToken, err)
	})
	t.Run("falha ao gravar o refresh não derruba o login", func(t *testing.T) {
		a := novoAmbiente(usuarioAtivo(1, "a@b.c", "s", kernel.RoleNormal))
		a.tokens.errInserir = errInfra
		s, err := a.casos().Login(ctx, cmd("s"))
		require.NoError(t, err)
		assert.True(t, s.Refresh.Vazio(), "sem refresh token, como no legado")
		assert.Equal(t, "jwt", s.Acesso)
	})
	t.Run("rehash: falha no hash ou no UPDATE só loga", func(t *testing.T) {
		for _, c := range []struct {
			nome          string
			errGerar      error
			errRehash     error
			wantRegravado int
		}{
			{"hash falha", errInfra, nil, 0},
			{"update falha", nil, errInfra, 1},
			{"migra", nil, nil, 1},
		} {
			t.Run(c.nome, func(t *testing.T) {
				a := novoAmbiente(usuarioAtivo(1, "a@b.c", "s", kernel.RoleNormal))
				a.hasher.rehash, a.hasher.errGerar, a.usuarios.errRehash = true, c.errGerar, c.errRehash
				_, err := a.casos().Login(ctx, cmd("s"))
				require.NoError(t, err)
				assert.Equal(t, c.wantRegravado, a.usuarios.rehash)
			})
		}
	})
}

func TestRenovarSessao_FalhaAoEmitirJWTNaRotacao(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(5, "a@b.c", "s", kernel.RoleNormal))
	a.tokens.porHash["hash:tok"] = &identidade.RefreshToken{ID: 9, UsuarioID: 5, HashToken: "hash:tok", ExpiraEm: a.agora.Add(time.Hour)}
	a.emissor = emissorFake{err: errInfra}
	_, err := a.casos().RenovarSessao(ctx, identidade.NovoTokenOpaco("tok"), identidadeapp.Origem{})
	assert.Equal(t, identidade.ErrGerarAccessToken, err)
	assert.Zero(t, a.tokens.inseridos, "nenhum token novo sai")
}

func TestLogout_ErroDeInfraSoLoga(t *testing.T) {
	a := novoAmbiente()
	a.tokens.errHash = errInfra
	assert.NotPanics(t, func() { a.casos().Logout(ctx, identidade.NovoTokenOpaco("x")) })
	a.casos().Logout(ctx, identidade.NovoTokenOpaco("")) // sem token: nada a fazer
}

func TestTrocarSenha_RamosDeFalha(t *testing.T) {
	cmd := func(atual, nova string) identidadeapp.ComandoTrocarSenha {
		return identidadeapp.ComandoTrocarSenha{SenhaAtual: identidade.NovaSenha(atual), NovaSenha: identidade.NovaSenha(nova), Captcha: "ok"}
	}
	u := usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal)
	p := principal(2, kernel.RoleNormal)
	casos := []struct {
		nome    string
		p       kernel.Principal
		cmd     identidadeapp.ComandoTrocarSenha
		ajustar func(a *ambiente)
		want    error
	}{
		{"Principal inválido falha fechado", kernel.Principal{}, cmd("Atual#123", "Nova#12345"), nil, kernel.ErrPrincipalInvalido},
		{"senha atual vazia", p, cmd("", "Nova#12345"), nil, identidade.ErrSenhaAtualObrigatoria},
		{"nova vazia", p, cmd("Atual#123", ""), nil, identidade.ErrNovaSenhaObrigatoria},
		{"rate limit", p, cmd("Atual#123", "Nova#12345"), func(a *ambiente) { a.limite = limitadorFake{bloquear: "reset:"} }, identidade.ErrMuitasTentativas},
		{"usuário sumiu → 404", p, cmd("Atual#123", "Nova#12345"), func(a *ambiente) { a.usuarios.errPorID = []error{identidade.ErrUsuarioNaoEncontrado} }, identidade.ErrUsuarioNaoEncontrado},
		{"infra na leitura → 500", p, cmd("Atual#123", "Nova#12345"), func(a *ambiente) { a.usuarios.errPorID = []error{errInfra} }, errInfra},
		{"infra no histórico → 500", p, cmd("Atual#123", "Nova#12345"), func(a *ambiente) { a.historico.errListar = errInfra }, errInfra},
		{"hash da nova falha → 500", p, cmd("Atual#123", "Nova#12345"), func(a *ambiente) { a.hasher.errGerar = errInfra }, errInfra},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			a := novoAmbiente(u)
			if c.ajustar != nil {
				c.ajustar(a)
			}
			assert.ErrorIs(t, a.casos().TrocarSenha(ctx, c.p, c.cmd), c.want)
		})
	}
	t.Run("falha ao gravar é 500 mesmo para inexistente (sem %w)", func(t *testing.T) {
		a := novoAmbiente(u)
		a.usuarios.errSenha = identidade.ErrUsuarioNaoEncontrado
		err := a.casos().TrocarSenha(ctx, p, cmd("Atual#123", "Nova#12345"))
		require.Error(t, err)
		assert.NotErrorIs(t, err, identidade.ErrUsuarioNaoEncontrado)
	})
	t.Run("histórico e revogação falham: troca segue (O8-iii)", func(t *testing.T) {
		a := novoAmbiente(u)
		a.historico.errRegistro, a.tokens.errTodos = errInfra, errInfra
		require.NoError(t, a.casos().TrocarSenha(ctx, p, cmd("Atual#123", "Nova#12345")))
		assert.Len(t, a.usuarios.senhas, 1)
	})
}

func TestAdminResetarSenha_Ramos(t *testing.T) {
	alvo := usuarioAtivo(5, "a@b.c", "Gerada#1234567", kernel.RoleNormal)
	admin := principal(1, kernel.RoleAdmin)
	cmd := identidadeapp.ComandoAdminResetarSenha{UsuarioID: 5}
	t.Run("não admin → 403; Principal inválido → 500; id <= 0 → 400", func(t *testing.T) {
		c := novoAmbiente(alvo).casos()
		_, err := c.AdminResetarSenha(ctx, principal(2, kernel.RoleNormal), cmd)
		assert.Equal(t, identidade.ErrAcessoRestritoAdmin, err)
		_, err = c.AdminResetarSenha(ctx, kernel.Principal{}, cmd)
		assert.Equal(t, kernel.ErrPrincipalInvalido, err)
		_, err = c.AdminResetarSenha(ctx, admin, identidadeapp.ComandoAdminResetarSenha{})
		assert.Equal(t, identidade.ErrUsuarioAlvoInvalido, err)
	})
	t.Run("senha gerada colide com a atual: regenera", func(t *testing.T) {
		a := novoAmbiente(alvo)
		a.senhas = &geradorSenhaFake{senhas: []string{"Gerada#1234567", "Outra#1234567"}}
		enviado, err := a.casos().AdminResetarSenha(ctx, admin, cmd)
		require.NoError(t, err)
		assert.True(t, enviado)
		require.Len(t, a.usuarios.senhas, 1)
		assert.Equal(t, "h:Outra#1234567", a.usuarios.senhas[0].Revelar(), "a senha gravada não é a atual")
	})
	t.Run("gerador ou hasher falham → 500 sem gravar", func(t *testing.T) {
		a := novoAmbiente(alvo)
		a.senhas.err = errInfra
		_, err := a.casos().AdminResetarSenha(ctx, admin, cmd)
		assert.ErrorIs(t, err, errInfra)
		b := novoAmbiente(alvo)
		b.hasher.errGerar = errInfra
		_, err = b.casos().AdminResetarSenha(ctx, admin, cmd)
		assert.ErrorIs(t, err, errInfra)
		assert.Empty(t, b.usuarios.senhas)
	})
	t.Run("falha no e-mail: reset fica, enviado=false", func(t *testing.T) {
		a := novoAmbiente(alvo)
		a.email = emailFake{err: errInfra}
		enviado, err := a.casos().AdminResetarSenha(ctx, admin, cmd)
		require.NoError(t, err)
		assert.False(t, enviado)
		assert.Equal(t, 1, a.historico.registros)
	})
}

func TestGestaoDeUsuarios_ExigeAdmin(t *testing.T) {
	c := novoAmbiente().casos()
	normal := principal(2, kernel.RoleNormal)
	o := identidade.OrdenacaoUsuarioPadrao()
	_, err := c.ListarUsuarios(ctx, normal, o, kernel.NovaPagina(1, 20))
	assert.Equal(t, identidade.ErrAcessoRestritoAdmin, err)
	_, _, err = c.CriarUsuario(ctx, normal, identidade.DadosUsuario{})
	assert.Equal(t, identidade.ErrAcessoRestritoAdmin, err)
	_, err = c.AtualizarUsuario(ctx, normal, 1, identidade.DadosUsuario{})
	assert.Equal(t, identidade.ErrAcessoRestritoAdmin, err)
	_, err = c.AlternarAtivoUsuario(ctx, normal, 1, nil)
	assert.Equal(t, identidade.ErrAcessoRestritoAdmin, err)
	_, err = c.ListarHistorico(ctx, normal, identidade.OrdenacaoHistoricoPadrao(), kernel.NovaPagina(1, 20))
	assert.Equal(t, identidade.ErrAcessoRestritoAdmin, err)
	_, err = c.ListarHistoricoDoUsuario(ctx, kernel.Principal{}, 1, identidade.OrdenacaoHistoricoPadrao(), kernel.NovaPagina(1, 20))
	assert.Equal(t, kernel.ErrPrincipalInvalido, err)
}

func TestCriarEAtualizarUsuario_Ramos(t *testing.T) {
	admin := principal(1, kernel.RoleAdmin)
	vend := int64(3)
	dados := identidade.DadosUsuario{Nome: "Ana", Email: "ana@rp.com", Role: "normal", IDVendedor: &vend}
	casos := []struct {
		nome    string
		ajustar func(a *ambiente)
		want    error
	}{
		{"vendedor: erro de infra", func(a *ambiente) { a.vendedores.errExiste = errInfra }, errInfra},
		{"vendedor inexistente → 400", func(a *ambiente) {}, identidade.ErrVendedorInexistente},
		{"gerador falha", func(a *ambiente) { a.vendedores.existe = true; a.senhas.err = errInfra }, errInfra},
		{"inserir falha", func(a *ambiente) { a.vendedores.existe = true; a.usuarios.errInserir = identidade.ErrEmailDuplicado }, identidade.ErrEmailDuplicado},
	}
	for _, c := range casos {
		t.Run("criar/"+c.nome, func(t *testing.T) {
			a := novoAmbiente()
			c.ajustar(a)
			_, _, err := a.casos().CriarUsuario(ctx, admin, dados)
			assert.ErrorIs(t, err, c.want)
		})
	}
	t.Run("criar/validação", func(t *testing.T) {
		_, _, err := novoAmbiente().casos().CriarUsuario(ctx, admin, identidade.DadosUsuario{})
		assert.Equal(t, identidade.ErrNomeObrigatorio, err)
	})
	t.Run("criar/releitura falha devolve o objeto em memória", func(t *testing.T) {
		a := novoAmbiente()
		u, enviado, err := a.casos().CriarUsuario(ctx, admin, identidade.DadosUsuario{Nome: "Ana", Email: "Ana@RP.com", Role: "normal"})
		require.NoError(t, err)
		assert.True(t, enviado)
		assert.Equal(t, "ana@rp.com", u.Email)
		assert.True(t, u.CriadoEm.IsZero())
	})

	existente := usuarioAtivo(7, "x@y.z", "s", kernel.RoleNormal)
	edicao := []struct {
		nome    string
		d       identidade.DadosUsuario
		ajustar func(a *ambiente)
		want    error
		nao     error
	}{
		{"validação", identidade.DadosUsuario{Nome: "A", Role: "x"}, func(*ambiente) {}, identidade.ErrRoleInvalido, nil},
		{"vendedor inexistente", identidade.DadosUsuario{Nome: "A", Role: "normal", IDVendedor: &vend}, func(*ambiente) {}, identidade.ErrVendedorInexistente, nil},
		{"atualizar: inexistente → 404", identidade.DadosUsuario{Nome: "A", Role: "normal"}, func(a *ambiente) { a.usuarios.errAtualiz = identidade.ErrUsuarioNaoEncontrado }, identidade.ErrUsuarioNaoEncontrado, nil},
		{"releitura falha → 500 (sem %w)", identidade.DadosUsuario{Nome: "A", Role: "normal"}, func(a *ambiente) { a.usuarios.errPorID = []error{identidade.ErrUsuarioNaoEncontrado} }, nil, identidade.ErrUsuarioNaoEncontrado},
	}
	for _, c := range edicao {
		t.Run("atualizar/"+c.nome, func(t *testing.T) {
			a := novoAmbiente(existente)
			c.ajustar(a)
			_, err := a.casos().AtualizarUsuario(ctx, admin, 7, c.d)
			require.Error(t, err)
			if c.want != nil {
				assert.ErrorIs(t, err, c.want)
			}
			if c.nao != nil {
				assert.NotErrorIs(t, err, c.nao)
			}
		})
	}
}

func TestAlternarAtivoUsuario_Ramos(t *testing.T) {
	admin := principal(1, kernel.RoleAdmin)
	a := novoAmbiente()
	_, err := a.casos().AlternarAtivoUsuario(ctx, admin, 9, nil)
	assert.Equal(t, identidade.ErrUsuarioNaoEncontrado, err)

	b := novoAmbiente(usuarioAtivo(9, "a@b.c", "s", kernel.RoleNormal))
	b.usuarios.errAtivo = errInfra
	_, err = b.casos().AlternarAtivoUsuario(ctx, admin, 9, nil)
	assert.ErrorIs(t, err, errInfra)

	c := novoAmbiente(usuarioAtivo(9, "a@b.c", "s", kernel.RoleNormal))
	c.tokens.errTodos = errInfra // revogação falha: só loga (I-09)
	u, err := c.casos().AlternarAtivoUsuario(ctx, admin, 9, nil)
	require.NoError(t, err)
	assert.False(t, u.Ativo)
}

func TestObterSessao_Ramos(t *testing.T) {
	vend := int64(4)
	normal := usuarioAtivo(2, "a@b.c", "s", kernel.RoleNormal)
	normal.IDVendedor = &vend
	_, _, err := novoAmbiente(normal).casos().ObterSessao(ctx, kernel.Principal{})
	assert.Equal(t, kernel.ErrPrincipalInvalido, err)

	a := novoAmbiente(normal)
	a.vendedores = vendedoresFake{desligado: true}
	_, desligado, err := a.casos().ObterSessao(ctx, principal(2, kernel.RoleNormal))
	require.NoError(t, err)
	assert.True(t, desligado)

	b := novoAmbiente(normal)
	b.vendedores = vendedoresFake{errDeslig: errInfra}
	_, _, err = b.casos().ObterSessao(ctx, principal(2, kernel.RoleNormal))
	assert.ErrorIs(t, err, errInfra)

	admin := usuarioAtivo(1, "adm@b.c", "s", kernel.RoleAdmin)
	admin.IDVendedor = &vend
	c := novoAmbiente(admin)
	c.vendedores = vendedoresFake{errDeslig: errInfra} // admin não consulta
	_, desligado, err = c.casos().ObterSessao(ctx, principal(1, kernel.RoleAdmin))
	require.NoError(t, err)
	assert.False(t, desligado)
}

func TestListagensEStatus(t *testing.T) {
	admin := principal(1, kernel.RoleAdmin)
	a := novoAmbiente(usuarioAtivo(1, "a@b.c", "s", kernel.RoleAdmin))
	a.historico.itens = []identidade.RegistroSenha{{ID: 1}}
	c := a.casos()
	r, err := c.ListarHistorico(ctx, admin, identidade.OrdenacaoHistoricoPadrao(), kernel.NovaPagina(1, 20))
	require.NoError(t, err)
	assert.Equal(t, 1, r.Total)
	r, err = c.ListarHistoricoDoUsuario(ctx, admin, 1, identidade.OrdenacaoHistoricoPadrao(), kernel.NovaPagina(1, 20))
	require.NoError(t, err)
	assert.Len(t, r.Itens, 1)
	st, err := c.StatusParaAutenticacao(ctx, 1)
	require.NoError(t, err)
	assert.True(t, st.Ativo)
}

func TestPublicadorLocal_EntregaATodosOsAssinantes(t *testing.T) {
	var recebidos []int64
	p := identidadeapp.NovoPublicadorLocal(
		func(_ context.Context, e identidade.ReusoDeRefreshDetectado) {
			recebidos = append(recebidos, e.TokenID)
		},
		func(_ context.Context, e identidade.ReusoDeRefreshDetectado) {
			recebidos = append(recebidos, e.UsuarioID)
		},
	)
	p.Publicar(ctx, identidade.ReusoDeRefreshDetectado{UsuarioID: 5, TokenID: 10})
	assert.Equal(t, []int64{10, 5}, recebidos)
	identidadeapp.SemPublicador{}.Publicar(ctx, identidade.ReusoDeRefreshDetectado{})
}

func TestResetarSenhasAtivos_ErroNaListagem(t *testing.T) {
	a := novoAmbiente()
	a.usuarios.ativos = []identidade.Usuario{*usuarioAtivo(1, "a@b.c", "s", kernel.RoleNormal)}
	a.usuarios.errPorID = []error{errors.New("caiu")}
	itens, err := a.casos().ResetarSenhasAtivos(ctx, false)
	assert.Error(t, err)
	assert.Empty(t, itens)
}
