package identidadehttp

import (
	"log"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

// UsuarioHandler trata /api/usuarios/* e /api/admin/reset-password
// (AdminOnly na rota; os casos de uso repetem a checagem de admin, I-05).
type UsuarioHandler struct {
	casos        *identidadeapp.Casos
	confiarProxy bool
}

// NovoUsuarioHandler exige os casos de uso (nil: panic na montagem).
func NovoUsuarioHandler(c *identidadeapp.Casos, confiarProxy bool) *UsuarioHandler {
	if c == nil {
		panic("identidadehttp: NovoUsuarioHandler exige os casos de uso")
	}
	return &UsuarioHandler{casos: c, confiarProxy: confiarProxy}
}

// ListUsuarios GET /api/usuarios?page&limit&order_by&order_dir (sem hash).
func (h *UsuarioHandler) ListUsuarios(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r)
	o := httpx.OrdenacaoDaQuery(r, identidade.CamposOrdenacaoUsuario(), identidade.OrdenacaoUsuarioPadrao())
	res, err := h.casos.ListarUsuarios(r.Context(), p, o, httpx.PaginaDaQuery(r))
	if err != nil {
		httpx.ResponderErro(w, "[usuarios] ListUsuarios", err)
		return
	}
	httpx.JSONPaginado(w, kernel.NovoResultado(listaDe(res.Itens, novoUsuarioListagem), res.Pagina, res.Total))
}

// CreateUsuario POST /api/usuarios → 201. A senha inicial vai só por e-mail.
func (h *UsuarioHandler) CreateUsuario(w http.ResponseWriter, r *http.Request) {
	const tag = "[usuarios] CreateUsuario"
	var req criarUsuarioRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r)
	u, enviado, err := h.casos.CriarUsuario(r.Context(), p, identidade.DadosUsuario{
		Nome: req.Nome, Email: req.Email, Role: req.Role, IDVendedor: req.IDVendedor,
	})
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	if !enviado {
		log.Printf("[usuarios] ATENÇÃO: usuário id=%d criado, mas email com senha inicial NÃO foi enviado", u.ID)
	}
	log.Printf("[usuarios] criado: id=%d por admin=%s email_enviado=%t", u.ID, p.Role(), enviado)
	httpx.JSON(w, http.StatusCreated, novoUsuarioCriado(u, enviado), "")
}

// UpdateUsuario PUT /api/usuarios/{id} (nome, role e vendedor).
func (h *UsuarioHandler) UpdateUsuario(w http.ResponseWriter, r *http.Request) {
	const tag = "[usuarios] UpdateUsuario"
	id, err := httpx.IDDoPath(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	var req atualizarUsuarioRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r)
	u, err := h.casos.AtualizarUsuario(r.Context(), p, id, identidade.DadosUsuario{Nome: req.Nome, Role: req.Role, IDVendedor: req.IDVendedor})
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	log.Printf("[usuarios] atualizado: id=%d por admin=%s", id, p.Role())
	httpx.JSON(w, http.StatusOK, novoUsuarioResposta(u), "")
}

// ToggleAtivoUsuario PATCH /api/usuarios/{id}/inativar; body opcional
// {"ativo": bool} (omitido = toggle). Inativar revoga as sessões.
func (h *UsuarioHandler) ToggleAtivoUsuario(w http.ResponseWriter, r *http.Request) {
	const tag = "[usuarios] ToggleAtivoUsuario"
	id, err := httpx.IDDoPath(r, "id")
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	ativo, err := httpx.LerAtivoOpcional(r)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r)
	u, err := h.casos.AlternarAtivoUsuario(r.Context(), p, id, ativo)
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	log.Printf("[usuarios] ativo=%t: id=%d por admin=%s", u.Ativo, id, p.Role())
	httpx.JSON(w, http.StatusOK, novoUsuarioResposta(u), "")
}

// AdminResetPassword POST /api/admin/reset-password {"usuario_id": n}: nova
// senha aleatória enviada por e-mail (nunca na resposta).
func (h *UsuarioHandler) AdminResetPassword(w http.ResponseWriter, r *http.Request) {
	const tag = "[usuarios] AdminResetPassword"
	var req adminResetRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r)
	enviado, err := h.casos.AdminResetarSenha(r.Context(), p, identidadeapp.ComandoAdminResetarSenha{
		UsuarioID: req.UsuarioID,
		Origem:    identidadeapp.Origem{IP: httpx.IPDoCliente(r, h.confiarProxy), UserAgent: r.UserAgent()},
	})
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.JSON(w, http.StatusOK, adminResetResposta{
		EmailEnviado: enviado, Mensagem: "Senha resetada e enviada por email ao usuário", Sucesso: true,
	}, "")
}
