package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/rotaperfumes/shared/kernel"
)

// ErrBodyJSONInvalido é o 400 de body que não decodifica no DTO.
var ErrBodyJSONInvalido = kernel.NovoErro(kernel.KindInvalido, "body JSON inválido")

// DecodificarJSON lê o body em dst. Qualquer falha vira ErrBodyJSONInvalido.
// Como o json.Decoder legado, lê só o primeiro valor JSON e aceita campos
// desconhecidos (contrato inalterado).
func DecodificarJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return ErrBodyJSONInvalido
	}
	return nil
}

// LerAtivoOpcional lê o body opcional { "ativo": bool } dos PATCH
// .../{id}/inativar (BUG-06):
//   - body ausente, vazio, `null`, `{}` ou `{"ativo":null}` → (nil, nil): toggle;
//   - `{"ativo":true|false}` → ponteiro para o valor;
//   - qualquer outro conteúdo (malformado, tipo errado, lixo após o objeto)
//     → ErrBodyJSONInvalido, e o handler responde 400 sem chamar o caso de uso.
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
		return nil, ErrBodyJSONInvalido
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrBodyJSONInvalido
	}
	return body.Ativo, nil
}

// IDDoPathPermissivo lê um wildcard numérico com o parse dos handlers
// legados (strconv.ParseInt): aceita sinal, zero e negativos, que seguem
// para o caso de uso e respondem 404. Preserva o contrato dos contextos
// migrados; IDDoPath (parse estrito, R-f) só vale onde a exceção X1 o
// aprovou. Falha → ErrIDInvalido ("id inválido").
func IDDoPathPermissivo(r *http.Request, nome string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(nome), 10, 64)
	if err != nil {
		return 0, ErrIDInvalido
	}
	return id, nil
}

// JSONPaginadoCom escreve 200 com os itens de r convertidos por conv (DTO)
// e a paginação. Sem itens, "data" continua null, como no legado.
func JSONPaginadoCom[T, D any](w http.ResponseWriter, r kernel.Resultado[T], conv func(T) D) {
	var itens []D
	for _, v := range r.Itens {
		itens = append(itens, conv(v))
	}
	JSONPaginado(w, kernel.NovoResultado(itens, r.Pagina, r.Total))
}
