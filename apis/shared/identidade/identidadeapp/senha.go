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
// short-circuit (conta) → hash (fora da tx) → numa tx (BUG-15): histórico,
// senha (com corte de sessão) e revogação dos refresh tokens. Falha em
// qualquer etapa desfaz tudo e é 500 (a senha não muda).
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

// gravarNovaSenha gera o hash (Argon2, fora da tx) e grava histórico,
// senha (sem obrigar troca, com corte de sessão) e revogação numa tx só.
func (c *Casos) gravarNovaSenha(ctx context.Context, u *identidade.Usuario, cmd ComandoTrocarSenha) error {
	novo, err := c.d.Hasher.Gerar(cmd.NovaSenha)
	if err != nil {
		log.Printf("[auth] reset-password: HashPassword: %v", err)
		return fmt.Errorf("identidadeapp: trocar senha: hash: %w", err)
	}
	g := gravacaoSenha{usuario: u, novo: novo, tipo: identidade.TipoResetUsuario, origem: cmd.Origem}
	if err := c.gravarSenhaNaTx(ctx, g); err != nil {
		log.Printf("[auth] reset-password: gravar senha: %v", err)
		return fmt.Errorf("identidadeapp: trocar senha: gravar: %v", err) // sem %w: inexistente aqui é 500
	}
	log.Printf("[auth] reset-password: user_id=%d", u.ID)
	return nil
}

// gravacaoSenha é a escrita atômica de uma troca/reset de senha: usuario
// traz o id e o hash anterior (histórico); por é o admin do reset (nil na
// troca pelo próprio usuário e no reset em massa).
type gravacaoSenha struct {
	usuario    *identidade.Usuario
	novo       identidade.HashSenha
	deveTrocar bool
	por        *int64
	tipo       identidade.TipoReset
	origem     Origem
}

// gravarSenhaNaTx grava, numa tx (BUG-15), o histórico, a nova senha (com o
// corte de sessão) e a revogação dos refresh tokens; qualquer erro faz
// rollback. Os logs de sucesso saem só depois do commit. AtualizarSenha
// volta sem embrulho (o chamador decide 404/500); histórico e revogação
// voltam com o nome da etapa.
func (c *Casos) gravarSenhaNaTx(ctx context.Context, g gravacaoSenha) error {
	id := g.usuario.ID
	err := c.d.TrocaSenha.WithTx(ctx, func(tx TxSenha) error {
		r := &identidade.RegistroSenha{UsuarioID: id, ResetadoPorID: g.por, HashAnterior: g.usuario.Hash,
			IP: g.origem.IP, UserAgent: g.origem.UserAgent, Tipo: g.tipo}
		if err := tx.Historico.Registrar(ctx, r); err != nil {
			return fmt.Errorf("identidadeapp: senha: histórico: %w", err)
		}
		if err := tx.Usuarios.AtualizarSenha(ctx, id, g.novo, g.deveTrocar); err != nil {
			return err
		}
		if err := tx.Tokens.RevogarTodosDoUsuario(ctx, id, identidade.MotivoSenha); err != nil {
			return fmt.Errorf("identidadeapp: senha: revogar refresh tokens: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	log.Printf("[senha-historico] registrado: usuario_id=%d tipo=%s", id, g.tipo)
	log.Printf("[refresh] todos tokens revogados: usuario_id=%d motivo=%s", id, identidade.MotivoSenha)
	return nil
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

// AdminResetarSenha gera uma senha aleatória para o usuário e, numa tx
// (BUG-15), registra o histórico (tipo admin), grava a senha (com troca
// obrigatória e corte de sessão) e revoga as sessões (I-09); o e-mail só
// sai depois do commit. A senha nunca é devolvida nem logada (M-02).
// emailEnviado=false não desfaz o reset.
func (c *Casos) AdminResetarSenha(ctx context.Context, p kernel.Principal, cmd ComandoAdminResetarSenha) (emailEnviado bool, err error) {
	defer vlog.Trace(arqSenha, "Identidade.AdminResetarSenha", "alvo=%d", cmd.UsuarioID)(&err)
	if err := exigirAdmin(p); err != nil {
		return false, err
	}
	if cmd.UsuarioID <= 0 {
		return false, identidade.ErrUsuarioAlvoInvalido
	}
	alvo, err := c.d.Usuarios.PorID(ctx, cmd.UsuarioID) // 404 e destino do e-mail
	if err != nil {
		return false, erroLeituraUsuario("[usuarios] AdminResetPassword: GetByID", err)
	}
	adminID := p.UserID()
	emailEnviado, err = c.redefinirSenhaGerada(ctx, alvo.ID, alvo.Email, alvo.Nome, &adminID, cmd.Origem)
	if err != nil {
		return false, err
	}
	if !emailEnviado {
		log.Printf("[usuarios] ATENÇÃO: senha de usuario_id=%d resetada, mas email NÃO foi enviado", alvo.ID)
	}
	log.Printf("[usuarios] admin resetou senha: usuario_id=%d por admin=%s email_enviado=%t", alvo.ID, p.Role(), emailEnviado)
	return emailEnviado, nil
}

// redefinirSenhaGerada é o passo comum do reset administrativo e do reset
// em massa (OPS-01), em três partes (BUG-15): (a) fora da tx, relê o
// usuário e gera senha + hash sem colidir com a atual; (b) numa tx,
// histórico (tipo admin, hash anterior = o relido), senha com troca
// obrigatória e revogação; (c) depois do commit, o e-mail.
func (c *Casos) redefinirSenhaGerada(ctx context.Context, id int64, email, nome string, por *int64, o Origem) (bool, error) {
	alvo, senha, hash, err := c.senhaGeradaPara(ctx, id)
	if err != nil {
		return false, err
	}
	g := gravacaoSenha{usuario: alvo, novo: hash, deveTrocar: true, por: por, tipo: identidade.TipoResetAdmin, origem: o}
	if err := c.gravarSenhaNaTx(ctx, g); err != nil {
		return false, erroGravacaoReset(err)
	}
	enviado := c.enviarSenhaInicial(ctx, email, nome, senha)
	log.Printf("[usuarios] admin reset: senha redefinida: id=%d email_enviado=%t", id, enviado)
	return enviado, nil
}

// erroGravacaoReset: usuário que sumiu entre a releitura e o UPDATE vira
// 404 (como no legado); o resto (histórico, revogação, begin/commit), 500
// com log.
func erroGravacaoReset(err error) error {
	if errors.Is(err, identidade.ErrUsuarioNaoEncontrado) {
		return identidade.ErrUsuarioNaoEncontrado
	}
	log.Printf("[usuarios] AdminResetPassword: gravar senha: %v", err)
	return fmt.Errorf("identidadeapp: reset de senha: gravar: %w", err)
}

// senhaGeradaPara relê o usuário e gera uma senha (e o hash) que não
// colida com a atual (até 3 tentativas). Roda fora da tx: o Argon2 não
// segura a conexão nem os locks.
func (c *Casos) senhaGeradaPara(ctx context.Context, id int64) (*identidade.Usuario, identidade.Senha, identidade.HashSenha, error) {
	alvo, err := c.d.Usuarios.PorID(ctx, id)
	if err != nil {
		return nil, identidade.Senha{}, identidade.HashSenha{}, erroLeituraUsuario("[usuarios] AdminResetPassword", err)
	}
	var senha identidade.Senha
	var hash identidade.HashSenha
	for tentativa := 1; tentativa <= maxTentativasSenhaGerada; tentativa++ {
		if senha, hash, err = c.gerarSenhaEHash(); err != nil {
			return nil, identidade.Senha{}, identidade.HashSenha{}, err
		}
		if !c.d.Hasher.Verificar(alvo.Hash, senha) {
			break
		}
		log.Printf("[usuarios] admin reset: senha gerada colidiu com a atual, regenerando: id=%d tentativa=%d", id, tentativa)
	}
	return alvo, senha, hash, nil
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
// devolvida como false. O texto do erro sai sem o e-mail: respostas SMTP
// 550 ecoam o RCPT (NB-1).
func (c *Casos) enviarSenhaInicial(ctx context.Context, email, nome string, senha identidade.Senha) bool {
	if err := c.d.Email.EnviarSenhaInicial(ctx, email, nome, senha.Revelar()); err != nil {
		log.Printf("[usuarios] falha ao enviar email de senha inicial para %s: %s", vlog.MaskEmail(email), vlog.OcultarEmail(err, email))
		return false
	}
	return true
}
