package httpx

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/rotaperfumes/shared/kernel"
)

// ErrIDInvalido é o 400 de id de path ausente, não numérico, ≤ 0 ou com "/".
var ErrIDInvalido = kernel.NovoErro(kernel.KindInvalido, "id inválido")

// PaginaDaQuery lê ?page= e ?limit= (F-10, regra única em kernel.NovaPagina).
func PaginaDaQuery(r *http.Request) kernel.Pagina {
	q := r.URL.Query()
	return PaginaDaQueryStrings(q.Get("page"), q.Get("limit"))
}

// PaginaDaQueryStrings é PaginaDaQuery a partir das strings cruas: valores
// não numéricos contam como ausentes (default).
func PaginaDaQueryStrings(page, limit string) kernel.Pagina {
	numero, _ := strconv.Atoi(page)
	limite, _ := strconv.Atoi(limit)
	return kernel.NovaPagina(numero, limite)
}

// OrdenacaoDaQuery lê ?order_by= e ?order_dir= contra o enum da entidade;
// valores fora do enum caem no padrão (C-02).
func OrdenacaoDaQuery[C ~string](r *http.Request, validos []C, padrao kernel.Ordenacao[C]) kernel.Ordenacao[C] {
	q := r.URL.Query()
	return kernel.ParseOrdenacao(q.Get("order_by"), q.Get("order_dir"), validos, padrao)
}

// BoolDaQuery lê um filtro booleano opcional: "true"/"false" (sem diferenciar
// maiúsculas, com trim); qualquer outro valor é "sem filtro" (nil).
func BoolDaQuery(r *http.Request, nome string) *bool {
	return ParseBoolOpcional(r.URL.Query().Get(nome))
}

// ParseBoolOpcional é BoolDaQuery sobre a string crua.
func ParseBoolOpcional(v string) *bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		v := true
		return &v
	case "false":
		v := false
		return &v
	default:
		return nil
	}
}

// IDDoPath lê um wildcard numérico do padrão da rota com parse estrito
// (R-f): obrigatório, decimal, > 0 e sem "/".
func IDDoPath(r *http.Request, nome string) (int64, error) {
	return ParseID(r.PathValue(nome))
}

// ParseID é o parse estrito de IDDoPath sobre a string crua: só dígitos
// ASCII (sem sinal, espaço ou "/"), cabendo em int64 e > 0.
func ParseID(v string) (int64, error) {
	if v == "" {
		return 0, ErrIDInvalido
	}
	for i := 0; i < len(v); i++ {
		if v[i] < '0' || v[i] > '9' {
			return 0, ErrIDInvalido
		}
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrIDInvalido
	}
	return id, nil
}
