// Package identidadehttp tem os handlers HTTP de Identidade e Acesso
// (ARQ-03): só decodificam, chamam identidadeapp e respondem. Os nomes de
// arquivo, tipos e métodos são os do legado, porque formam o rótulo do vlog
// de rota (TST-05: "[auth_handler.go] [AuthHandler.Login]").
package identidadehttp

import (
	"time"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
)

// Requests (as mesmas chaves do legado).
type (
	loginRequest struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		CaptchaToken string `json:"captchaToken"`
	}
	refreshRequest struct {
		RefreshToken string `json:"refresh_token"`
	}
	resetPasswordRequest struct {
		SenhaAtual   string `json:"senha_atual"`
		NovaSenha    string `json:"nova_senha"`
		CaptchaToken string `json:"captchaToken"`
	}
	criarUsuarioRequest struct {
		Nome       string `json:"nome"`
		Email      string `json:"email"`
		Role       string `json:"role"`
		IDVendedor *int64 `json:"id_vendedor"`
	}
	atualizarUsuarioRequest struct {
		Nome       string `json:"nome"`
		Role       string `json:"role"`
		IDVendedor *int64 `json:"id_vendedor"`
	}
	adminResetRequest struct {
		UsuarioID int64 `json:"usuario_id"`
	}
)

// Respostas. Campos em ordem alfabética: o JSON sai byte a byte igual ao
// map[string]any do legado (o encoder ordena as chaves do map). Nenhum DTO
// tem campo de hash (I-12).
type (
	usuarioLogin struct {
		Email string `json:"email"`
		ID    int64  `json:"id"`
		Nome  string `json:"nome"`
		Role  string `json:"role"`
	}
	loginResposta struct {
		ExpiresIn   int64        `json:"expires_in"`
		TokenType   string       `json:"token_type"`
		TrocarSenha bool         `json:"trocar_senha"`
		User        usuarioLogin `json:"user"`
	}
	refreshResposta struct {
		ExpiresIn int64  `json:"expires_in"`
		TokenType string `json:"token_type"`
	}
	mensagemResposta struct {
		Mensagem string `json:"mensagem"`
	}
	meResposta struct {
		Ativo             bool       `json:"ativo"`
		CreatedAt         time.Time  `json:"created_at"`
		Email             string     `json:"email"`
		ID                int64      `json:"id"`
		IDVendedor        *int64     `json:"id_vendedor"`
		Nome              string     `json:"nome"`
		Role              string     `json:"role"`
		TrocarSenha       bool       `json:"trocar_senha"`
		UltimoLoginAt     *time.Time `json:"ultimo_login_at"`
		VendedorDesligado bool       `json:"vendedor_desligado"`
	}
	// usuarioListagem é o item de GET /api/usuarios (sem updated_at).
	usuarioListagem struct {
		Ativo         bool       `json:"ativo"`
		CreatedAt     time.Time  `json:"created_at"`
		Email         string     `json:"email"`
		ID            int64      `json:"id"`
		IDVendedor    *int64     `json:"id_vendedor"`
		Nome          string     `json:"nome"`
		Role          string     `json:"role"`
		UltimoLoginAt *time.Time `json:"ultimo_login_at"`
		VendedorNome  string     `json:"vendedor_nome"`
	}
	// usuarioResposta é o usuário de create/update/toggle.
	usuarioResposta struct {
		Ativo         bool       `json:"ativo"`
		CreatedAt     time.Time  `json:"created_at"`
		Email         string     `json:"email"`
		ID            int64      `json:"id"`
		IDVendedor    *int64     `json:"id_vendedor"`
		Nome          string     `json:"nome"`
		Role          string     `json:"role"`
		UltimoLoginAt *time.Time `json:"ultimo_login_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
		VendedorNome  string     `json:"vendedor_nome"`
	}
	usuarioCriado struct {
		Ativo         bool       `json:"ativo"`
		CreatedAt     time.Time  `json:"created_at"`
		Email         string     `json:"email"`
		EmailEnviado  bool       `json:"email_enviado"`
		ID            int64      `json:"id"`
		IDVendedor    *int64     `json:"id_vendedor"`
		Nome          string     `json:"nome"`
		Role          string     `json:"role"`
		UltimoLoginAt *time.Time `json:"ultimo_login_at"`
		UpdatedAt     time.Time  `json:"updated_at"`
		VendedorNome  string     `json:"vendedor_nome"`
	}
	adminResetResposta struct {
		EmailEnviado bool   `json:"email_enviado"`
		Mensagem     string `json:"mensagem"`
		Sucesso      bool   `json:"sucesso"`
	}
	// historicoItem nunca tem o hash anterior (I-12); resetado_por_* só
	// aparecem quando o reset foi feito por outro usuário.
	historicoItem struct {
		CreatedAt       time.Time `json:"created_at"`
		ID              int64     `json:"id"`
		IPOrigem        string    `json:"ip_origem"`
		ResetadoPorID   *int64    `json:"resetado_por_id,omitempty"`
		ResetadoPorNome *string   `json:"resetado_por_nome,omitempty"`
		TipoReset       string    `json:"tipo_reset"`
		UserAgent       string    `json:"user_agent"`
		UsuarioID       int64     `json:"usuario_id"`
		UsuarioNome     string    `json:"usuario_nome"`
	}
)

const tokenType = "Bearer"

func novoLoginResposta(s *identidadeapp.SessaoEmitida, ttl time.Duration) loginResposta {
	u := s.Usuario
	return loginResposta{
		ExpiresIn:   int64(ttl.Seconds()),
		TokenType:   tokenType,
		TrocarSenha: u.DeveTrocarSenha,
		User:        usuarioLogin{Email: u.Email, ID: u.ID, Nome: u.Nome, Role: u.Role.String()},
	}
}

func novoMeResposta(u *identidade.Usuario, desligado bool) meResposta {
	return meResposta{
		Ativo: u.Ativo, CreatedAt: u.CriadoEm, Email: u.Email, ID: u.ID, IDVendedor: u.IDVendedor,
		Nome: u.Nome, Role: u.Role.String(), TrocarSenha: u.DeveTrocarSenha, UltimoLoginAt: u.UltimoLoginEm,
		VendedorDesligado: desligado,
	}
}

func novoUsuarioListagem(u identidade.Usuario) usuarioListagem {
	return usuarioListagem{
		Ativo: u.Ativo, CreatedAt: u.CriadoEm, Email: u.Email, ID: u.ID, IDVendedor: u.IDVendedor,
		Nome: u.Nome, Role: u.Role.String(), UltimoLoginAt: u.UltimoLoginEm, VendedorNome: textoOuVazio(u.VendedorNome),
	}
}

func novoUsuarioResposta(u *identidade.Usuario) usuarioResposta {
	return usuarioResposta{
		Ativo: u.Ativo, CreatedAt: u.CriadoEm, Email: u.Email, ID: u.ID, IDVendedor: u.IDVendedor,
		Nome: u.Nome, Role: u.Role.String(), UltimoLoginAt: u.UltimoLoginEm, UpdatedAt: u.AtualizadoEm,
		VendedorNome: textoOuVazio(u.VendedorNome),
	}
}

func novoUsuarioCriado(u *identidade.Usuario, emailEnviado bool) usuarioCriado {
	r := novoUsuarioResposta(u)
	return usuarioCriado{
		Ativo: r.Ativo, CreatedAt: r.CreatedAt, Email: r.Email, EmailEnviado: emailEnviado, ID: r.ID,
		IDVendedor: r.IDVendedor, Nome: r.Nome, Role: r.Role, UltimoLoginAt: r.UltimoLoginAt,
		UpdatedAt: r.UpdatedAt, VendedorNome: r.VendedorNome,
	}
}

func novoHistoricoItem(h identidade.RegistroSenha) historicoItem {
	return historicoItem{
		CreatedAt: h.CriadoEm, ID: h.ID, IPOrigem: h.IP, ResetadoPorID: h.ResetadoPorID,
		ResetadoPorNome: h.ResetadoPorNome, TipoReset: string(h.Tipo), UserAgent: h.UserAgent,
		UsuarioID: h.UsuarioID, UsuarioNome: h.UsuarioNome,
	}
}

// listaDe converte os itens sem nunca devolver nil: estas listagens sempre
// responderam "data": [] (o legado montava make([]map, 0, n)).
func listaDe[T, D any](itens []T, conv func(T) D) []D {
	out := make([]D, 0, len(itens))
	for _, v := range itens {
		out = append(out, conv(v))
	}
	return out
}

func textoOuVazio(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
