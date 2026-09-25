// Package handlers contém handlers HTTP.
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// msgBodyJSONInvalido é a mensagem devolvida com 400 quando o body não é um
// JSON válido para o endpoint.
const msgBodyJSONInvalido = "body JSON inválido"

// errBodyAtivoInvalido indica body preenchido, mas inválido, em PATCH /inativar.
var errBodyAtivoInvalido = errors.New("handlers: body de ativo inválido")

// ativoBody é o formato aceito pelos endpoints PATCH .../{id}/inativar.
type ativoBody struct {
	Ativo *bool `json:"ativo"`
}

// decodeAtivoOpcional lê o body opcional { "ativo": bool } dos endpoints
// PATCH .../{id}/inativar (clientes, produtos e usuários) — BUG-06.
//
//   - body vazio, `null`, `{}` ou `{"ativo":null}` → (nil, nil): toggle;
//   - `{"ativo":true|false}` → ponteiro para o valor: define o estado;
//   - qualquer outro conteúdo (JSON malformado, tipo errado como
//     `{"ativo":"false"}` ou `{"ativo":1}`, ou lixo após o objeto) →
//     errBodyAtivoInvalido. O chamador responde 400 sem chamar o service.
func decodeAtivoOpcional(r *http.Request) (*bool, error) {
	if r.Body == nil {
		return nil, nil
	}
	dec := json.NewDecoder(r.Body)
	var body ativoBody
	if err := dec.Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, errBodyAtivoInvalido
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errBodyAtivoInvalido
	}
	return body.Ativo, nil
}

// lerAtivoOpcional aplica decodeAtivoOpcional e, em caso de body inválido,
// já responde 400 "body JSON inválido". Retorna ok=false nesse caso.
func lerAtivoOpcional(w http.ResponseWriter, r *http.Request) (ativo *bool, ok bool) {
	ativo, err := decodeAtivoOpcional(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, msgBodyJSONInvalido)
		return nil, false
	}
	return ativo, true
}
