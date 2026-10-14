package identidadeapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqSenha = "senha.go"

// ComandoTrocarSenha é o POST /api/auth/reset-password (o usuário vem do
// Principal, nunca do body).
type ComandoTrocarSenha struct {
	SenhaAtual identidade.Senha
	NovaSenha  identidade.Senha
	Captcha    string
	Origem     Origem
}

// TrocarSenha troca a senha do próprio usuário. Ordem exata do legado
// (I-12): obrigatórios → captcha → rate limit → senha atual (conta) → nova
// == atual → força (não conta) → histórico, atual + 2 últimas, sem
// short-circuit (conta) → grava histórico e senha (com corte de sessão) →
// revoga os refresh tokens. NÃO é transacional (O8-iii, card BUG-15): a
// sequência de chamadas é a do legado.
func (c *Casos) TrocarSenha(ctx context.Context, p kernel.Principal, cmd ComandoTrocarSenha) (err error) {
	defer vlog.Trace(arqSenha, "Identidade.TrocarSenha", "user_id=%d", p.UserID())(&err)
	if !p.Valido() {
		return kernel.ErrPrincipalInvalido
	}
	if cmd.SenhaAtual.Vazia() {
		return identidade.ErrSenhaAtualObrigatoria
	}
	if cmd.NovaSenha.Vazia() {
		return identidade.ErrNovaSenhaObrigatoria
	}
	uid, ip := p.UserID(), cmd.Origem.IP
	if err := c.verificarCaptcha(ctx, cmd.Captcha, ip, "reset-password"); err != nil {
		return err
	}
	chave := "reset:" + ip + "|" + strconv.FormatInt(uid, 10)
	if bloqueado, retry := c.d.Limites.Reset.Bloqueado(chave); bloqueado {
		log.Printf("[auth] reset-password: bloqueado por rate limit: user_id=%d retry_after=%s", uid, retry)
		return bloqueio(retry)
	}
	u, err := c.d.Usuarios.PorID(ctx, uid)
	if err != nil {
		return erroLeituraUsuario("[auth] reset-password: GetByID", err)
	}
	if err := c.validarNovaSenha(ctx, u, cmd, chave); err != nil {
		return err
	}
	return c.gravarNovaSenha(ctx, u, cmd)
}

// validarNovaSenha confere senha atual, igualdade, força e histórico.
func (c *Casos) validarNovaSenha(ctx context.Context, u *identidade.Usuario, cmd ComandoTrocarSenha, chave string) error {
	if !c.d.Hasher.Verificar(u.Hash, cmd.SenhaAtual) {
		log.Printf("[auth] reset-password: senha atual incorreta: user_id=%d", u.ID)
		c.d.Limites.Reset.RegistrarFalha(chave)
		return identidade.ErrSenhaAtualIncorreta
	}
	c.d.Limites.Reset.RegistrarSucesso(chave)
	// Igualdade antes da força: uma senha atual fraca não mascara o erro real.
	if cmd.NovaSenha.Igual(cmd.SenhaAtual) {
		log.Printf("[auth] reset-password: rejeitada: user_id=%d motivo=igual_senha_atual", u.ID)
		return identidade.ErrNovaSenhaIgualAtual
	}
	if err := identidade.ValidarForca(cmd.NovaSenha); err != nil {
		log.Printf("[auth] reset-password: rejeitada: user_id=%d motivo=senha_fraca", u.ID)
		return err
	}
	ultimas, err := c.d.Historico.DoUsuario(ctx, u.ID, identidade.OrdenacaoHistoricoPadrao(),
		kernel.NovaPagina(1, identidade.UltimasSenhasNoHistorico))
	if err != nil {
		log.Printf("[auth] reset-password: ListarPorUsuario: %v", err)
		return fmt.Errorf("identidadeapp: trocar senha: histórico: %w", err)
	}
	hashes := []identidade.HashSenha{u.Hash}
	for _, r := range ultimas.Itens {
		hashes = append(hashes, r.HashAnterior)
	}
	if identidade.SenhaReutilizada(cmd.NovaSenha, hashes, c.d.Hasher) {
		log.Printf("[auth] reset-password: rejeitada: user_id=%d motivo=senha_reutilizada", u.ID)
		c.d.Limites.Reset.RegistrarFalha(chave)
		return identidade.ErrSenhaReutilizada
	}
	return nil
}

// gravarNovaSenha grava histórico (falha só loga), a senha (sem obrigar
// troca, com corte de sessão) e revoga os refresh tokens (falha só loga).
func (c *Casos) gravarNovaSenha(ctx context.Context, u *identidade.Usuario, cmd ComandoTrocarSenha) error {
	novo, err := c.d.Hasher.Gerar(cmd.NovaSenha)
	if err != nil {
		log.Printf("[auth] reset-password: HashPassword: %v", err)
		return fmt.Errorf("identidadeapp: trocar senha: hash: %w", err)
	}
	c.registrarHistorico(ctx, u, nil, identidade.TipoResetUsuario, cmd.Origem,
		"[auth] reset-password: falha ao registrar histórico de senha do user_id")
	if err := c.d.Usuarios.AtualizarSenha(ctx, u.ID, novo, false); err != nil {
		log.Printf("[auth] reset-password: UpdatePasswordHash: %v", err)
		return fmt.Errorf("identidadeapp: trocar senha: gravar: %v", err) // sem %w: inexistente aqui é 500
	}
	c.revogarTodos(ctx, u.ID, identidade.MotivoSenha, "[auth] reset-password")
	log.Printf("[auth] reset-password: user_id=%d", u.ID)
	return nil
}

// registrarHistorico grava a senha anterior em senha_historico. Falha só é
// logada: a troca segue (comportamento do legado).
func (c *Casos) registrarHistorico(ctx context.Context, u *identidade.Usuario, por *int64, tipo identidade.TipoReset, o Origem, tagFalha string) {
	r := &identidade.RegistroSenha{UsuarioID: u.ID, ResetadoPorID: por, HashAnterior: u.Hash, IP: o.IP, UserAgent: o.UserAgent, Tipo: tipo}
	if err := c.d.Historico.Registrar(ctx, r); err != nil {
		log.Printf("%s=%d: %v", tagFalha, u.ID, err)
		return
	}
	log.Printf("[senha-historico] registrado: usuario_id=%d tipo=%s", u.ID, tipo)
}

// erroLeituraUsuario: inexistente vira 404; o resto, 500 com log.
func erroLeituraUsuario(tag string, err error) error {
	if errors.Is(err, identidade.ErrUsuarioNaoEncontrado) {
		return identidade.ErrUsuarioNaoEncontrado
	}
	log.Printf("%s: %v", tag, err)
	return fmt.Errorf("identidadeapp: ler usuário: %w", err)
}

// ComandoAdminResetarSenha é o POST /api/admin/reset-password.
type ComandoAdminResetarSenha struct {
	UsuarioID int64
	Origem    Origem
}

// AdminResetarSenha gera uma senha aleatória para o usuário, grava (com
// troca obrigatória e corte de sessão), envia por e-mail, registra no
// histórico (tipo admin) e revoga as sessões (I-09). A senha nunca é
// devolvida nem logada (M-02). emailEnviado=false não desfaz o reset.
func (c *Casos) AdminResetarSenha(ctx context.Context, p kernel.Principal, cmd ComandoAdminResetarSenha) (emailEnviado bool, err error) {
	defer vlog.Trace(arqSenha, "Identidade.AdminResetarSenha", "alvo=%d", cmd.UsuarioID)(&err)
	if err := exigirAdmin(p); err != nil {
		return false, err
	}
	if cmd.UsuarioID <= 0 {
		return false, identidade.ErrUsuarioAlvoInvalido
	}
	alvo, err := c.d.Usuarios.PorID(ctx, cmd.UsuarioID) // hash anterior para a auditoria
	if err != nil {
		return false, erroLeituraUsuario("[usuarios] AdminResetPassword: GetByID", err)
	}
	emailEnviado, err = c.redefinirSenhaGerada(ctx, alvo.ID, alvo.Email, alvo.Nome)
	if err != nil {
		return false, err
	}
	adminID := p.UserID()
	c.registrarHistorico(ctx, alvo, &adminID, identidade.TipoResetAdmin, cmd.Origem,
		"[usuarios] AdminResetPassword: falha ao registrar histórico de senha do usuario_id")
	c.revogarTodos(ctx, alvo.ID, identidade.MotivoSenha, "[usuarios] AdminResetPassword")
	if !emailEnviado {
		log.Printf("[usuarios] ATENÇÃO: senha de usuario_id=%d resetada, mas email NÃO foi enviado", alvo.ID)
	}
	log.Printf("[usuarios] admin resetou senha: usuario_id=%d por admin=%s email_enviado=%t", alvo.ID, p.Role(), emailEnviado)
	return emailEnviado, nil
}

// redefinirSenhaGerada relê o usuário, gera uma senha que não colida com a
// atual (até 3 tentativas), grava com troca obrigatória e envia por e-mail.
// É o passo comum do reset administrativo e do reset em massa (OPS-01).
func (c *Casos) redefinirSenhaGerada(ctx context.Context, id int64, email, nome string) (bool, error) {
	alvo, err := c.d.Usuarios.PorID(ctx, id)
	if err != nil {
		return false, erroLeituraUsuario("[usuarios] AdminResetPassword", err)
	}
	var senha identidade.Senha
	var hash identidade.HashSenha
	for tentativa := 1; tentativa <= maxTentativasSenhaGerada; tentativa++ {
		if senha, hash, err = c.gerarSenhaEHash(); err != nil {
			return false, err
		}
		if !c.d.Hasher.Verificar(alvo.Hash, senha) {
			break
		}
		log.Printf("[usuarios] admin reset: senha gerada colidiu com a atual, regenerando: id=%d tentativa=%d", id, tentativa)
	}
	if err := c.d.Usuarios.AtualizarSenha(ctx, id, hash, true); err != nil {
		return false, erroLeituraUsuario("[usuarios] AdminResetPassword: UpdatePasswordHash", err)
	}
	enviado := c.enviarSenhaInicial(ctx, email, nome, senha)
	log.Printf("[usuarios] admin reset: senha redefinida: id=%d email_enviado=%t", id, enviado)
	return enviado, nil
}

// gerarSenhaEHash gera a senha aleatória (para o e-mail) e o hash (banco).
func (c *Casos) gerarSenhaEHash() (identidade.Senha, identidade.HashSenha, error) {
	senha, err := c.d.Senhas.Gerar()
	if err != nil {
		return identidade.Senha{}, identidade.HashSenha{}, fmt.Errorf("identidadeapp: gerar senha aleatória: %w", err)
	}
	hash, err := c.d.Hasher.Gerar(senha)
	if err != nil {
		return identidade.Senha{}, identidade.HashSenha{}, fmt.Errorf("identidadeapp: hash da senha gerada: %w", err)
	}
	return senha, hash, nil
}

// enviarSenhaInicial é best-effort: falha só é logada (e-mail mascarado) e
// devolvida como false.
func (c *Casos) enviarSenhaInicial(ctx context.Context, email, nome string, senha identidade.Senha) bool {
	if err := c.d.Email.EnviarSenhaInicial(ctx, email, nome, senha.Revelar()); err != nil {
		log.Printf("[usuarios] falha ao enviar email de senha inicial para %s: %v", vlog.MaskEmail(email), err)
		return false
	}
	return true
}
