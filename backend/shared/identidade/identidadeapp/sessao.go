package identidadeapp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/vlog"
)

const arqSessao = "sessao.go"

// RenovarSessao é o POST /api/auth/refresh (I-06, I-07, SEC-04/07/12).
// Contam no rate limit por IP: token inexistente, expirado e revogado fora
// da janela de graça. NÃO contam: revogado dentro da janela e a corrida na
// rotação (rotação concorrente legítima). Nenhum token é emitido em erro.
func (c *Casos) RenovarSessao(ctx context.Context, token identidade.TokenOpaco, o Origem) (s *SessaoEmitida, err error) {
	defer vlog.Trace(arqSessao, "Identidade.RenovarSessao", "")(&err)
	if token.Vazio() {
		return nil, identidade.ErrRefreshObrigatorio
	}
	chave := "refresh-ip:" + o.IP
	if bloqueado, retry := c.d.Limites.Refresh.Bloqueado(chave); bloqueado {
		log.Printf("[auth] refresh: IP bloqueado por rate limit: ip=%s retry_after=%s", o.IP, retry)
		return nil, bloqueio(retry)
	}
	rt, err := c.validarRefresh(ctx, token, o)
	if err != nil {
		if contaNoRateLimit(err) {
			c.d.Limites.Refresh.RegistrarFalha(chave)
		}
		return nil, err
	}
	u, err := c.d.Usuarios.PorID(ctx, rt.UsuarioID)
	if err != nil {
		log.Printf("[auth] refresh: GetByID: %v", err)
		return nil, fmt.Errorf("identidadeapp: refresh: ler usuário: %v", err) // sem %w: inexistente aqui é 500
	}
	if !u.Ativo {
		c.recusarRefreshInativo(ctx, rt, u.ID, chave)
		return nil, identidade.ErrRefreshRevogado
	}
	s, err = c.rotacionar(ctx, rt, u, o)
	if err != nil {
		return nil, err
	}
	c.d.Limites.Refresh.RegistrarSucesso(chave)
	log.Printf("[auth] refresh OK: user_id=%d", u.ID)
	return s, nil
}

// recusarRefreshInativo trata o refresh de usuário inativo (SEC-22): revoga
// o token apresentado (best-effort) e conta falha no rate limit por IP. A
// resposta é a de token revogado, sem revelar que a conta está inativa.
func (c *Casos) recusarRefreshInativo(ctx context.Context, rt *identidade.RefreshToken, usuarioID int64, chave string) {
	log.Printf("[auth] refresh: usuário inativo, token revogado: user_id=%d", usuarioID)
	if err := c.d.Tokens.Revogar(ctx, rt.ID, identidade.MotivoInativacao); err != nil {
		log.Printf("[auth] refresh: revogar token de usuário inativo user_id=%d: %v", usuarioID, err)
	}
	c.d.Limites.Refresh.RegistrarFalha(chave)
}

// errRevogadoForaDaJanela distingue, só para o rate limit, o revogado fora
// da janela (conta) do revogado recente (não conta). Embrulha o sentinela
// público, então a resposta HTTP é idêntica (SEC-04).
var errRevogadoForaDaJanela = fmt.Errorf("fora da janela de graça: %w", identidade.ErrRefreshRevogado)

func contaNoRateLimit(err error) bool {
	return errors.Is(err, identidade.ErrRefreshInvalido) || errors.Is(err, identidade.ErrRefreshExpirado) ||
		errors.Is(err, errRevogadoForaDaJanela)
}

// validarRefresh busca o token pelo hash e aplica RefreshToken.Avaliar
// (expiração antes de revogação; RevogadoRecente antes de Revogado, I-07).
func (c *Casos) validarRefresh(ctx context.Context, token identidade.TokenOpaco, o Origem) (*identidade.RefreshToken, error) {
	rt, err := c.d.Tokens.PorHash(ctx, c.d.Refresh.Hash(token))
	if errors.Is(err, identidade.ErrRefreshInvalido) {
		log.Printf("[auth] refresh: token não encontrado")
		return nil, err
	}
	if err != nil {
		log.Printf("[auth] refresh: ValidateRefreshToken: %v", err)
		return nil, fmt.Errorf("identidadeapp: refresh: buscar token: %w", err)
	}
	switch estado := rt.Avaliar(c.agora()); estado {
	case identidade.RefreshExpirado:
		log.Printf("[auth] refresh: token expirado")
		return nil, identidade.ErrRefreshExpirado
	case identidade.RefreshRevogadoRecente:
		log.Printf("[auth] refresh: token revogado recentemente (corrida entre abas), não conta no rate limit: ip=%s", o.IP)
		return nil, identidade.ErrRefreshRevogado
	case identidade.RefreshRevogado:
		c.tratarRevogadoForaDaJanela(ctx, rt, estado, o)
		return nil, errRevogadoForaDaJanela
	default:
		vlog.Printf(arqSessao, "Identidade.RenovarSessao", "decisão refresh válido token_id=%d", rt.ID)
		return rt, nil
	}
}

// rotacionar revoga o token antigo (UPDATE condicional), emite o JWT e grava
// o novo refresh na MESMA transação (I-06, UoW §5.2). Qualquer erro dá
// rollback e nenhum token sai; a corrida (0 linhas) é 401 sem contar.
func (c *Casos) rotacionar(ctx context.Context, rt *identidade.RefreshToken, u *identidade.Usuario, o Origem) (*SessaoEmitida, error) {
	s := &SessaoEmitida{}
	err := c.d.Rotacao.WithTx(ctx, func(tx TxRotacao) error {
		if err := tx.Tokens.Revogar(ctx, rt.ID, identidade.MotivoRotacao); err != nil {
			return err
		}
		acesso, err := c.d.Emissor.Emitir(u.ID, u.Role)
		if err != nil {
			log.Printf("[auth] refresh: GenerateJWT: %v", err)
			return identidade.ErrGerarAccessToken
		}
		s.Acesso = acesso
		s.Refresh, err = c.emitirRefresh(ctx, tx.Tokens, u.ID, o)
		return err
	})
	switch {
	case errors.Is(err, identidade.ErrRefreshRevogado):
		log.Printf("[refresh] rotação recusada: token já revogado id=%d", rt.ID)
		log.Printf("[auth] refresh: token já revogado por requisição concorrente (corrida no BeginRotation), não conta no rate limit: user_id=%d ip=%s", u.ID, o.IP)
		return nil, identidade.ErrRefreshRevogado
	case errors.Is(err, identidade.ErrGerarAccessToken):
		return nil, err
	case err != nil:
		log.Printf("[auth] refresh: rotação: %v", err)
		return nil, fmt.Errorf("identidadeapp: refresh: rotação: %w", err)
	}
	log.Printf("[refresh] token rotacionado: antigo_id=%d usuario_id=%d", rt.ID, u.ID)
	return s, nil
}

// tratarRevogadoForaDaJanela decide pelo motivo gravado (SEC-07). Motivo
// "rotacao" é reuso de token rotacionado (possível roubo): marca o reuso de
// forma atômica (SEC-12) e, se for o primeiro na janela de supressão, corta
// as sessões e publica o evento. Demais motivos (ou NULL): só log. A
// resposta HTTP é a mesma em todos os casos.
//
// Fronteira transacional idêntica à do legado (O8-iv): a marca é um UPDATE
// isolado; erro ao marcar corta assim mesmo (fail-closed); corte incompleto
// desfaz a marca (compensação), para o próximo reuso tentar de novo.
func (c *Casos) tratarRevogadoForaDaJanela(ctx context.Context, rt *identidade.RefreshToken, estado identidade.EstadoRefresh, o Origem) {
	if !rt.ReusoDeRotacao(estado) {
		log.Printf("[auth] refresh: token revogado apresentado fora da janela de graça: user_id=%d token_id=%d motivo=%s ip=%s",
			rt.UsuarioID, rt.ID, motivoParaLog(rt.Motivo), o.IP)
		return
	}
	cortar, marca, err := c.RegistrarReuso(ctx, rt.ID)
	if err != nil {
		log.Printf("[auth][seguranca] refresh: falha ao registrar reuso (fail-closed, corta assim mesmo): token_id=%d: %v", rt.ID, err)
		cortar, marca = true, time.Time{}
	}
	if !cortar {
		log.Printf("[auth][seguranca] refresh: reuso repetido de token rotacionado dentro da janela de supressão — sessões NÃO revogadas de novo: user_id=%d token_id=%d ip=%s ua=%s",
			rt.UsuarioID, rt.ID, o.IP, o.UserAgent)
		return
	}
	c.cortarSessoes(ctx, rt, marca, o)
	c.d.Publicador.Publicar(ctx, identidade.ReusoDeRefreshDetectado{UsuarioID: rt.UsuarioID, TokenID: rt.ID, IP: o.IP, UserAgent: o.UserAgent})
}

// cortarSessoes revoga todos os refresh tokens do usuário e grava o corte de
// sessão (derruba os access tokens já emitidos, SEC-08).
func (c *Casos) cortarSessoes(ctx context.Context, rt *identidade.RefreshToken, marca time.Time, o Origem) {
	log.Printf("[auth][seguranca] refresh: reuso de refresh token já rotacionado fora da janela de graça (possível roubo de token) — revogando todas as sessões: user_id=%d token_id=%d revoked_at=%s ip=%s ua=%s",
		rt.UsuarioID, rt.ID, rt.RevogadoEm.Format(time.RFC3339), o.IP, o.UserAgent)
	falhou := false
	if err := c.d.Tokens.RevogarTodosDoUsuario(ctx, rt.UsuarioID, identidade.MotivoRevogacaoMassa); err != nil {
		falhou = true
		log.Printf("[auth][seguranca] refresh: falha na revogação em massa após reuso: user_id=%d: %v", rt.UsuarioID, err)
	}
	if err := c.d.Usuarios.InvalidarSessoes(ctx, rt.UsuarioID, identidade.CorteDeSessao(c.agora())); err != nil {
		falhou = true
		log.Printf("[auth][seguranca] refresh: falha ao invalidar access tokens após reuso: user_id=%d: %v", rt.UsuarioID, err)
	}
	if !falhou || marca.IsZero() {
		return
	}
	if err := c.DesfazerReuso(ctx, rt.ID, marca); err != nil {
		log.Printf("[auth][seguranca] refresh: falha ao desfazer a marca de reuso após corte incompleto: token_id=%d: %v", rt.ID, err)
		return
	}
	log.Printf("[auth][seguranca] refresh: corte incompleto — marca de reuso desfeita (o próximo reuso tenta cortar de novo): token_id=%d", rt.ID)
}

// motivoParaLog: NULL (legado) vira "desconhecido(legado)".
func motivoParaLog(m identidade.MotivoRevogacao) string {
	if m == identidade.MotivoDesconhecido {
		return "desconhecido(legado)"
	}
	return string(m)
}

// Logout é o POST /api/auth/logout: revoga o refresh token informado (motivo
// logout) e nunca falha (idempotente, I-09). Token inexistente ou já
// revogado não é logado como erro.
func (c *Casos) Logout(ctx context.Context, token identidade.TokenOpaco) {
	var err error
	defer vlog.Trace(arqSessao, "Identidade.Logout", "token=%t", !token.Vazio())(&err)
	if token.Vazio() {
		return
	}
	rt, err := c.d.Tokens.PorHash(ctx, c.d.Refresh.Hash(token))
	if err == nil {
		err = c.d.Tokens.Revogar(ctx, rt.ID, identidade.MotivoLogout)
	}
	switch {
	case err == nil:
		log.Printf("[refresh] token revogado: id=%d usuario_id=%d motivo=%s", rt.ID, rt.UsuarioID, identidade.MotivoLogout)
	case errors.Is(err, identidade.ErrRefreshInvalido), errors.Is(err, identidade.ErrRefreshRevogado):
		err = nil
	default:
		log.Printf("[auth] logout: RevokeToken: %v", err)
	}
}

// RegistrarReuso marca, de forma atômica, o reuso do token rotacionado
// tokenID (SEC-12). cortar=true quando é o primeiro reuso ou o anterior saiu
// da janela de supressão; marca é o instante gravado (relógio do Go,
// truncado ao segundo como a coluna DATETIME), usado por DesfazerReuso.
// Sem corte, a marca é zero. Em erro, (false, zero, err): o chamador decide
// o fail-closed.
func (c *Casos) RegistrarReuso(ctx context.Context, tokenID int64) (cortar bool, marca time.Time, err error) {
	agora := c.agora().Truncate(time.Second)
	cortar, err = c.d.Tokens.MarcarReuso(ctx, tokenID, agora, agora.Add(-c.o.JanelaSupressaoReuso))
	if err != nil || !cortar {
		return false, time.Time{}, err
	}
	return true, agora, nil
}

// DesfazerReuso limpa a marca gravada por RegistrarReuso, se ainda for a
// mesma (compensação do corte incompleto, I-07).
func (c *Casos) DesfazerReuso(ctx context.Context, tokenID int64, marca time.Time) error {
	return c.d.Tokens.DesfazerReuso(ctx, tokenID, marca)
}
