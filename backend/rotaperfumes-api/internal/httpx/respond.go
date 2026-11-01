package httpx

import (
	"encoding/json"
	"net/http"

	"github.com/rotaperfumes/shared/kernel"
)

// envelope é a resposta padrão {success, data, error, pagination}.
//
// Os campos estão em ordem alfabética de propósito: o JSON sai byte a byte
// igual ao do map[string]any legado (o encoder ordena as chaves do map).
type envelope struct {
	Data       any        `json:"data,omitempty"`
	Error      string     `json:"error,omitempty"`
	Pagination *Paginacao `json:"pagination,omitempty"`
	Success    bool       `json:"success"`
}

// Paginacao é o bloco "pagination" das listagens (campos em ordem
// alfabética, mesma razão do envelope).
type Paginacao struct {
	Limit int `json:"limit"`
	Page  int `json:"page"`
	Pages int `json:"pages"`
	Total int `json:"total"`
}

// JSON escreve o envelope. errMsg vazio = sucesso; data nil não é emitido.
// errMsg deve ser sempre uma mensagem constante (F-05).
func JSON(w http.ResponseWriter, status int, data any, errMsg string) {
	escrever(w, status, envelope{Data: data, Error: errMsg, Success: errMsg == ""})
}

// JSONComPaginacao escreve {success:true, data, pagination}; data é sempre
// emitido (inclusive null), como no writeJSONWithPagination legado.
func JSONComPaginacao(w http.ResponseWriter, status int, data any, p Paginacao) {
	escrever(w, status, envelopePaginado{Data: data, Pagination: p, Success: true})
}

// JSONPaginado escreve 200 com os itens e a paginação de um Resultado.
func JSONPaginado[T any](w http.ResponseWriter, r kernel.Resultado[T]) {
	JSONComPaginacao(w, http.StatusOK, r.Itens, Paginacao{
		Limit: r.Pagina.Limite(),
		Page:  r.Pagina.Numero(),
		Pages: r.Paginas(),
		Total: r.Total,
	})
}

// SemConteudo escreve 204 sem corpo (RFC 9110 §15.3.5).
func SemConteudo(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// envelopePaginado difere do envelope só por "data" sem omitempty.
type envelopePaginado struct {
	Data       any       `json:"data"`
	Pagination Paginacao `json:"pagination"`
	Success    bool      `json:"success"`
}

func escrever(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}
