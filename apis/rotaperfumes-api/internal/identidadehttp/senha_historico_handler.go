package identidadehttp

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

// SenhaHistoricoHandler trata /api/senha-historico (AdminOnly). A resposta
// nunca traz o hash anterior (I-12).
type SenhaHistoricoHandler struct{ casos *identidadeapp.Casos }

// NovoSenhaHistoricoHandler exige os casos de uso (nil: panic na montagem).
func NovoSenhaHistoricoHandler(c *identidadeapp.Casos) *SenhaHistoricoHandler {
	if c == nil {
		panic("identidadehttp: NovoSenhaHistoricoHandler exige os casos de uso")
	}
	return &SenhaHistoricoHandler{casos: c}
}

// ListarTodos GET /api/senha-historico?page&limit&order_by&order_dir
func (h *SenhaHistoricoHandler) ListarTodos(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r)
	res, err := h.casos.ListarHistorico(r.Context(), p, ordenacaoHistorico(r), httpx.PaginaDaQuery(r))
	responderHistorico(w, "[senha-historico] ListarTodos", res, err)
}

// ListarPorUsuario GET /api/senha-historico/{user_id...}: vazio → 400
// "obrigatório"; parse estrito, decimal > 0 sem "/" (X1, R-f).
func (h *SenhaHistoricoHandler) ListarPorUsuario(w http.ResponseWriter, r *http.Request) {
	const tag = "[senha-historico] ListarPorUsuario"
	bruto := r.PathValue("user_id")
	if bruto == "" {
		httpx.ResponderErro(w, tag, identidade.ErrHistoricoSemUsuarioID)
		return
	}
	usuarioID, err := httpx.ParseID(bruto)
	if err != nil {
		httpx.ResponderErro(w, tag, identidade.ErrHistoricoUsuarioID)
		return
	}
	p, _ := httpx.PrincipalFrom(r)
	res, err := h.casos.ListarHistoricoDoUsuario(r.Context(), p, usuarioID, ordenacaoHistorico(r), httpx.PaginaDaQuery(r))
	responderHistorico(w, tag, res, err)
}

func ordenacaoHistorico(r *http.Request) kernel.Ordenacao[identidade.CampoHistorico] {
	return httpx.OrdenacaoDaQuery(r, identidade.CamposOrdenacaoHistorico(), identidade.OrdenacaoHistoricoPadrao())
}

func responderHistorico(w http.ResponseWriter, tag string, res kernel.Resultado[identidade.RegistroSenha], err error) {
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.JSONPaginado(w, kernel.NovoResultado(listaDe(res.Itens, novoHistoricoItem), res.Pagina, res.Total))
}
