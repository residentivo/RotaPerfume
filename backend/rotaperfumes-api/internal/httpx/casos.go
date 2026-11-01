package httpx

import (
	"net/http"

	"github.com/rotaperfumes/shared/kernel"
)

// Apoio dos handlers finos de todos os contextos (ARQ-05: promovidos do
// comercialhttp, lição 4 do ARQ-02): ler o {id}, adiar o decode do body e
// responder o resultado do caso de uso pelo mapper.

// IDOuErro lê o {nome} com o parse estrito de IDDoPath (CHORE-03): zero,
// negativo, sinal, espaço, não numérico ou overflow → responde 400 "id
// inválido" (antes do escopo e de qualquer SQL) e devolve ok=false.
func IDOuErro(w http.ResponseWriter, r *http.Request, tag, nome string) (int64, bool) {
	id, err := IDDoPath(r, nome)
	if err != nil {
		ResponderErro(w, tag, err)
		return 0, false
	}
	return id, true
}

// LeitorJSON adia o decode do body para depois da autorização do caso de
// uso ("escopo antes do decode", E-05). Body inválido → ErrBodyJSONInvalido
// (400). O valor é atribuível ao Leitor[D] de cada <ctx>app.
func LeitorJSON[Req, D any](r *http.Request, conv func(Req) D) func() (D, error) {
	return func() (D, error) {
		var req Req
		if err := DecodificarJSON(r, &req); err != nil {
			var zero D
			return zero, err
		}
		return conv(req), nil
	}
}

// Responder escreve o erro pelo mapper ou o objeto convertido no status.
func Responder[T, D any](w http.ResponseWriter, tag string, status int, v *T, err error, conv func(T) D) {
	if err != nil {
		ResponderErro(w, tag, err)
		return
	}
	JSON(w, status, conv(*v), "")
}

// ResponderPaginado escreve a listagem paginada (nil → null; vazia → []).
func ResponderPaginado[T, D any](w http.ResponseWriter, tag string, res kernel.Resultado[T], err error, conv func(T) D) {
	if err != nil {
		ResponderErro(w, tag, err)
		return
	}
	JSONPaginadoCom(w, res, conv)
}

// ResponderSemConteudo escreve o erro pelo mapper ou 204 (DELETE).
func ResponderSemConteudo(w http.ResponseWriter, tag string, err error) {
	if err != nil {
		ResponderErro(w, tag, err)
		return
	}
	SemConteudo(w)
}
