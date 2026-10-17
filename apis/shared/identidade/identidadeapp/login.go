package identidadeapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/vlog"
)

const arqLogin = "login.go"

// ComandoLogin é o POST /api/auth/login.
type ComandoLogin struct {
	Email   string
	Senha   identidade.Senha
	Captcha string
	Origem  Origem
}

// SessaoEmitida são os tokens de uma sessão nova (login) ou renovada
// (refresh). Os tokens vão só para cookies HttpOnly (I-15).
type SessaoEmitida struct {
	Usuario *identidade.Usuario // nil no refresh
	Acesso  string
	// Refresh pode vir vazio no login se a gravação falhou (o login segue,
	// como no legado).
	Refresh identidade.TokenOpaco
}

// Login autentica por e-mail e senha. Ordem (I-13, I-14, I-16): captcha →
// rate limit (IP e IP+conta) → credencial. Inexistente e senha errada dão o
// mesmo 401; inexistente roda o hash descartável.
func (c *Casos) Login(ctx context.Context, cmd ComandoLogin) (s *SessaoEmitida, err error) {
	defer vlog.Trace(arqLogin, "Identidade.Login", "")(&err)
	email := strings.TrimSpace(cmd.Email)
	if email == "" || cmd.Senha.Vazia() {
		return nil, identidade.ErrEmailESenhaObrigatorios
	}
	mascarado := vlog.MaskEmail(email)
	log.Printf("[auth] Login attempt: email=%s", mascarado)

	ip := cmd.Origem.IP
	if err := c.verificarCaptcha(ctx, cmd.Captcha, ip, "login"); err != nil {
		return nil, err
	}
	chaveIP, chaveConta := "ip:"+ip, "acct:"+ip+"|"+email
	if bloqueado, retry := c.d.Limites.Login.Bloqueado(chaveIP); bloqueado {
		log.Printf("[auth] login: IP bloqueado por rate limit: ip=%s retry_after=%s", ip, retry)
		return nil, bloqueio(retry)
	}
	if bloqueado, retry := c.d.Limites.Login.Bloqueado(chaveConta); bloqueado {
		log.Printf("[auth] login: conta bloqueada por rate limit: email=%s retry_after=%s", mascarado, retry)
		return nil, bloqueio(retry)
	}

	u, err := c.credencial(ctx, email, mascarado, cmd.Senha)
	if err != nil {
		if errors.Is(err, identidade.ErrCredenciaisInvalidas) || errors.Is(err, identidade.ErrUsuarioInativo) {
			c.d.Limites.Login.RegistrarFalha(chaveIP)
			c.d.Limites.Login.RegistrarFalha(chaveConta)
		}
		return nil, err
	}
	c.d.Limites.Login.RegistrarSucesso(chaveIP)
	c.d.Limites.Login.RegistrarSucesso(chaveConta)
	return c.abrirSessao(ctx, u, cmd.Senha, cmd.Origem)
}

// credencial busca o usuário e confere ativo e senha.
func (c *Casos) credencial(ctx context.Context, email, mascarado string, senha identidade.Senha) (*identidade.Usuario, error) {
	u, err := c.d.Usuarios.PorEmail(ctx, email)
	if errors.Is(err, identidade.ErrUsuarioNaoEncontrado) {
		log.Printf("[auth] login: usuário não encontrado: %s", mascarado)
		c.d.Hasher.ExecutarDescartavel(senha) // mesmo custo de um login real (I-16)
		return nil, identidade.ErrCredenciaisInvalidas
	}
	if err != nil {
		log.Printf("[auth] login: repo.GetByEmail: %v", err)
		return nil, fmt.Errorf("identidadeapp: login: %w", err)
	}
	if !u.Ativo {
		log.Printf("[auth] login: usuário inativo: %s", mascarado)
		return nil, identidade.ErrUsuarioInativo
	}
	if !c.d.Hasher.Verificar(u.Hash, senha) {
		log.Printf("[auth] login: senha incorreta para: %s", mascarado)
		return nil, identidade.ErrCredenciaisInvalidas
	}
	return u, nil
}

// abrirSessao migra o hash se preciso, emite o JWT e o refresh e marca o
// último login (os dois últimos passos não derrubam o login).
func (c *Casos) abrirSessao(ctx context.Context, u *identidade.Usuario, senha identidade.Senha, o Origem) (*SessaoEmitida, error) {
	c.rehashSeNecessario(ctx, u, senha)
	acesso, err := c.d.Emissor.Emitir(u.ID, u.Role)
	if err != nil {
		log.Printf("[auth] login: GenerateJWT: %v", err)
		return nil, identidade.ErrGerarToken
	}
	refresh, err := c.emitirRefresh(ctx, c.d.Tokens, u.ID, o)
	if err != nil {
		log.Printf("[auth] login: GenerateRefreshToken: %v", err)
	}
	_ = c.d.Usuarios.RegistrarLogin(ctx, u.ID, c.agora())
	log.Printf("[auth] login OK: user_id=%d role=%s trocar_senha=%t", u.ID, u.Role, u.DeveTrocarSenha)
	return &SessaoEmitida{Usuario: u, Acesso: acesso, Refresh: refresh}, nil
}

// rehashSeNecessario migra bcrypt legado ou Argon2id com parâmetros antigos
// para o Argon2id atual (SEC-13). Best-effort e condicional ao hash lido;
// não mexe no corte de sessão.
func (c *Casos) rehashSeNecessario(ctx context.Context, u *identidade.Usuario, senha identidade.Senha) {
	if !c.d.Hasher.PrecisaRehash(u.Hash) {
		return
	}
	novo, err := c.d.Hasher.Gerar(senha)
	if err != nil {
		log.Printf("[auth] login: rehash: HashPassword: %v", err)
		return
	}
	if err := c.d.Usuarios.RegravarHash(ctx, u.ID, u.Hash, novo); err != nil {
		log.Printf("[auth] login: rehash user_id=%d: %v", u.ID, err)
		return
	}
	log.Printf("[auth] login: hash de senha migrado para argon2id: user_id=%d", u.ID)
}
