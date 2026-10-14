// Package handlers contém handlers HTTP.
package handlers

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
)

// msgBodyJSONInvalido é a mensagem devolvida com 400 quando o body não é um
// JSON válido para o endpoint.
const msgBodyJSONInvalido = "body JSON inválido"

// lerAtivoOpcional lê o body opcional { "ativo": bool } dos PATCH
// .../{id}/inativar (BUG-06) e, em caso de body inválido, já responde 400
// "body JSON inválido". Retorna ok=false nesse caso.
//
// ARQ-02: delega a httpx.LerAtivoOpcional (regra única, usada também pelo
// catalogohttp); este helper some quando o último handler legado for
// migrado.
func lerAtivoOpcional(w http.ResponseWriter, r *http.Request) (ativo *bool, ok bool) {
	ativo, err := httpx.LerAtivoOpcional(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, msgBodyJSONInvalido)
		return nil, false
	}
	return ativo, true
}
