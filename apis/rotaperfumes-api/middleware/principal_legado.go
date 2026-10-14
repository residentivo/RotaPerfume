// Package middleware é a ponte de leitura do Principal para os handlers
// legados de Comercial, Vendas e Analytics (ARQ-03). O middleware JWT
// legado saiu: a autenticação é do httpx.Autenticador, e GetUserID/GetRole
// apenas LEEM o Principal que ele gravou (sem setter algum, O4). Sai no
// ARQ-07, com o último handler legado.
package middleware

import (
	"context"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
)

// GetUserID devolve o user_id do request autenticado.
func GetUserID(ctx context.Context) (int64, bool) {
	p, ok := httpx.PrincipalDoContexto(ctx)
	if !ok {
		return 0, false
	}
	return p.UserID(), true
}

// GetRole devolve o role do BANCO do request autenticado (I-05).
func GetRole(ctx context.Context) (string, bool) {
	p, ok := httpx.PrincipalDoContexto(ctx)
	if !ok {
		return "", false
	}
	return p.Role().String(), true
}
