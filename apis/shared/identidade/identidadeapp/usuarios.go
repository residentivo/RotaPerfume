package identidadeapp

import (
	"context"
	"fmt"
	"log"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqUsuarios = "usuarios.go"

// ListarUsuarios pagina os usuários (admin, I-05).
func (c *Casos) ListarUsuarios(ctx context.Context, p kernel.Principal, o kernel.Ordenacao[identidade.CampoUsuario], pg kernel.Pagina) (r kernel.Resultado[identidade.Usuario], err error) {
	defer vlog.Trace(arqUsuarios, "Identidade.ListarUsuarios", "page=%d limit=%d ordem=%s", pg.Numero(), pg.Limite(), o.Campo())(&err)
	if err := exigirAdmin(p); err != nil {
		return kernel.Resultado[identidade.Usuario]{}, err
	}
	return c.d.Usuarios.Listar(ctx, o, pg)
}

// CriarUsuario cria o usuário com senha aleatória enviada por e-mail (nunca
// devolvida nem logada, M-02) e troca obrigatória no primeiro acesso.
// enviado=false (e-mail não saiu) não desfaz a criação. Devolve o registro relido
// (BUG-11); se só a releitura falhar, o objeto em memória.
func (c *Casos) CriarUsuario(ctx context.Context, p kernel.Principal, d identidade.DadosUsuario) (u *identidade.Usuario, enviado bool, err error) {
	defer vlog.Trace(arqUsuarios, "Identidade.CriarUsuario", "")(&err)
	if err := exigirAdmin(p); err != nil {
		return nil, false, err
	}
	v, err := identidade.ValidarCriacao(d)
	if err != nil {
		return nil, false, err
	}
	if err := c.validarVendedor(ctx, v.IDVendedor); err != nil {
		return nil, false, err
	}
	senha, hash, err := c.gerarSenhaEHash()
	if err != nil {
		return nil, false, err
	}
	u = identidade.NovoUsuario(v, hash)
	if err := c.d.Usuarios.Inserir(ctx, u); err != nil {
		return nil, false, err
	}
	// Dados em memória: o e-mail sai mesmo que a releitura falhe (BUG-11).
	enviado = c.enviarSenhaInicial(ctx, u.Email, u.Nome, senha)
	vlog.Printf(arqUsuarios, "Identidade.CriarUsuario", "decisão criado id=%d email=%s role=%s email_enviado=%t",
		u.ID, vlog.MaskEmail(u.Email), u.Role, enviado)
	return c.relerCriado(ctx, u), enviado, nil
}

// relerCriado relê o usuário recém-gravado (timestamps e vendedor_nome do
// banco, BUG-11). O INSERT já foi confirmado: falha aqui só loga e devolve
// o objeto em memória.
func (c *Casos) relerCriado(ctx context.Context, u *identidade.Usuario) *identidade.Usuario {
	gravado, err := c.d.Usuarios.PorID(ctx, u.ID)
	if err != nil {
		log.Printf("[usuarios] criado, mas falhou a releitura: id=%d: %v", u.ID, err)
		return u
	}
	return gravado
}

// validarVendedor confere o vendedor vinculado (nil = sem vínculo).
func (c *Casos) validarVendedor(ctx context.Context, idVendedor *int64) error {
	if idVendedor == nil {
		return nil
	}
	existe, err := c.d.Vendedores.Existe(ctx, *idVendedor)
	if err != nil {
		return fmt.Errorf("identidadeapp: vendedor vinculado: %w", err)
	}
	if !existe {
		return identidade.ErrVendedorInexistente
	}
	return nil
}

// AtualizarUsuario edita nome, role e vendedor e devolve o registro relido.
// Falha na releitura é 500, mesmo "não encontrado" (como no legado).
func (c *Casos) AtualizarUsuario(ctx context.Context, p kernel.Principal, id int64, d identidade.DadosUsuario) (u *identidade.Usuario, err error) {
	defer vlog.Trace(arqUsuarios, "Identidade.AtualizarUsuario", "id=%d", id)(&err)
	if err := exigirAdmin(p); err != nil {
		return nil, err
	}
	v, err := identidade.ValidarEdicao(d)
	if err != nil {
		return nil, err
	}
	if err := c.validarVendedor(ctx, v.IDVendedor); err != nil {
		return nil, err
	}
	if err := c.d.Usuarios.Atualizar(ctx, id, v); err != nil {
		return nil, err
	}
	u, err = c.d.Usuarios.PorID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("identidadeapp: releitura após atualizar: %v", err) // sem %w: vira 500
	}
	return u, nil
}

// AlternarAtivoUsuario aplica o PATCH /inativar: ativo nil inverte. Inativar
// grava o corte de sessão (adapter) e revoga os refresh tokens (I-09); a
// falha na revogação só é logada (o Autenticador já barra o inativo).
func (c *Casos) AlternarAtivoUsuario(ctx context.Context, p kernel.Principal, id int64, ativo *bool) (u *identidade.Usuario, err error) {
	defer vlog.Trace(arqUsuarios, "Identidade.AlternarAtivoUsuario", "id=%d toggle=%t", id, ativo == nil)(&err)
	if err := exigirAdmin(p); err != nil {
		return nil, err
	}
	u, err = c.d.Usuarios.PorID(ctx, id)
	if err != nil {
		return nil, err
	}
	novo := !u.Ativo
	if ativo != nil {
		novo = *ativo
	}
	if err := c.d.Usuarios.DefinirAtivo(ctx, id, novo); err != nil {
		return nil, err
	}
	u.Ativo = novo
	if !novo {
		c.revogarTodos(ctx, id, identidade.MotivoInativacao, "[usuarios] ToggleAtivoUsuario")
	}
	vlog.Printf(arqUsuarios, "Identidade.AlternarAtivoUsuario", "decisão ativo=%t id=%d", novo, id)
	return u, nil
}

// ObterSessao é o GET /api/auth/me: o usuário do Principal e se o vendedor
// vinculado (não admin) está desligado. Consulta própria a cada chamada.
func (c *Casos) ObterSessao(ctx context.Context, p kernel.Principal) (u *identidade.Usuario, vendedorDesligado bool, err error) {
	defer vlog.Trace(arqUsuarios, "Identidade.ObterSessao", "user_id=%d", p.UserID())(&err)
	if !p.Valido() {
		return nil, false, kernel.ErrPrincipalInvalido
	}
	u, err = c.d.Usuarios.PorID(ctx, p.UserID())
	if err != nil {
		return nil, false, erroLeituraUsuario("[auth] me: GetByID", err)
	}
	idVendedor, vinculado := u.VendedorVinculado()
	if u.EhAdmin() || !vinculado {
		return u, false, nil
	}
	desligado, err := c.d.Vendedores.Desligado(ctx, idVendedor)
	if err != nil {
		log.Printf("[auth] me: vendedor desligado: %v", err)
		return nil, false, fmt.Errorf("identidadeapp: me: vendedor desligado: %w", err)
	}
	return u, desligado, nil
}

// StatusParaAutenticacao é a consulta do httpx.Autenticador a cada request
// protegido (SEC-06/SEC-08): sem cache (R4). Inexistente devolve
// identidade.ErrUsuarioNaoEncontrado.
func (c *Casos) StatusParaAutenticacao(ctx context.Context, userID int64) (st identidade.StatusAcesso, err error) {
	defer vlog.Trace(arqUsuarios, "Identidade.StatusParaAutenticacao", "user_id=%d", userID)(&err)
	return c.d.Usuarios.StatusParaAutenticacao(ctx, userID)
}

// ListarHistorico pagina todo o histórico de senhas (admin).
func (c *Casos) ListarHistorico(ctx context.Context, p kernel.Principal, o kernel.Ordenacao[identidade.CampoHistorico], pg kernel.Pagina) (r kernel.Resultado[identidade.RegistroSenha], err error) {
	defer vlog.Trace(arqUsuarios, "Identidade.ListarHistorico", "page=%d limit=%d ordem=%s", pg.Numero(), pg.Limite(), o.Campo())(&err)
	if err := exigirAdmin(p); err != nil {
		return kernel.Resultado[identidade.RegistroSenha]{}, err
	}
	return c.d.Historico.Todos(ctx, o, pg)
}

// ListarHistoricoDoUsuario pagina o histórico de um usuário (admin).
func (c *Casos) ListarHistoricoDoUsuario(ctx context.Context, p kernel.Principal, usuarioID int64, o kernel.Ordenacao[identidade.CampoHistorico], pg kernel.Pagina) (r kernel.Resultado[identidade.RegistroSenha], err error) {
	defer vlog.Trace(arqUsuarios, "Identidade.ListarHistoricoDoUsuario", "usuario_id=%d page=%d limit=%d ordem=%s", usuarioID, pg.Numero(), pg.Limite(), o.Campo())(&err)
	if err := exigirAdmin(p); err != nil {
		return kernel.Resultado[identidade.RegistroSenha]{}, err
	}
	return c.d.Historico.DoUsuario(ctx, usuarioID, o, pg)
}
