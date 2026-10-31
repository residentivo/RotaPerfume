package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/rotaperfumes/shared/kernel"
)

// ErrBodyJSONInvalido é o 400 de body que não decodifica no DTO.
var ErrBodyJSONInvalido = kernel.NovoErro(kernel.KindInvalido, "body JSON inválido")

// ErrCorpoMuitoGrande é o 413 de corpo acima do limite da rota (SEC-14).
var ErrCorpoMuitoGrande = kernel.NovoErro(kernel.KindMuitoGrande, "corpo da requisição excede o limite de 256 KB")

// DecodificarJSON lê o body em dst. Corpo acima do limite da rota (o
// MaxBytesReader do Router) vira ErrCorpoMuitoGrande; qualquer outra falha,
// ErrBodyJSONInvalido. Como o json.Decoder legado, lê só o primeiro valor
// JSON e aceita campos desconhecidos (contrato inalterado).
func DecodificarJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return erroDeCorpo(err)
	}
	return nil
}

// erroDeCorpo traduz a falha de leitura/decode do body no sentinela público.
func erroDeCorpo(err error) error {
	var excedeu *http.MaxBytesError
	if errors.As(err, &excedeu) {
		return ErrCorpoMuitoGrande
	}
	return ErrBodyJSONInvalido
}

// LerAtivoOpcional lê o body opcional { "ativo": bool } dos PATCH
// .../{id}/inativar (BUG-06):
//   - body ausente, vazio, `null`, `{}` ou `{"ativo":null}` → (nil, nil): toggle;
//   - `{"ativo":true|false}` → ponteiro para o valor;
//   - qualquer outro conteúdo (malformado, tipo errado, lixo após o objeto)
//     → ErrBodyJSONInvalido, e o handler responde 400 sem chamar o caso de uso;
//   - corpo acima do limite da rota → ErrCorpoMuitoGrande (413, SEC-14).
func LerAtivoOpcional(r *http.Request) (*bool, error) {
	if r.Body == nil {
		return nil, nil
	}
	dec := json.NewDecoder(r.Body)
	var body struct {
		Ativo *bool `json:"ativo"`
	}
	if err := dec.Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, erroDeCorpo(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, erroDeCorpo(err)
	}
	return body.Ativo, nil
}

// JSONPaginadoCom escreve 200 com os itens de r convertidos por conv (DTO)
// e a paginação. Preserva nil × vazio, como o legado: sem linhas no banco
// (Itens nil) "data" é null; a listagem vazia de quem não tem carteira
// (Itens não nulo e vazio, E-03) é [].
func JSONPaginadoCom[T, D any](w http.ResponseWriter, r kernel.Resultado[T], conv func(T) D) {
	JSONPaginado(w, kernel.NovoResultado(Lista(r.Itens, conv), r.Pagina, r.Total))
}

// Lista converte os itens preservando nil × vazio (nil → null; vazio → []).
func Lista[T, D any](itens []T, conv func(T) D) []D {
	if itens == nil {
		return nil
	}
	out := make([]D, 0, len(itens))
	for _, v := range itens {
		out = append(out, conv(v))
	}
	return out
}

// Int64DaQuery lê um id opcional da query com o parse legado
// (parseInt64Query): espaços aparados; ausente ou inválido = 0 (sem filtro).
func Int64DaQuery(r *http.Request, nome string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get(nome)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
