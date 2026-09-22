// Package handlers contém handlers HTTP.
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/repositories"
)

// vendedorScope resume a restrição de carteira aplicada à requisição atual.
//
// Restrito=false (usuário admin): sem restrição, a consulta enxerga todos os
// registros.
//
// Restrito=true (usuário role=normal): a consulta deve ser limitada a
// VendedorID. Quando o usuário normal não tem vendedor vinculado
// (id_vendedor NULL em usuarios), VendedorID vem 0 — nesse caso o chamador
// deve tratar como "nenhum resultado" (nunca deixar de filtrar).
type vendedorScope struct {
	Restrito   bool
	VendedorID int64
}

// SemAcesso reporta se o escopo é restrito e o usuário não tem vendedor
// vinculado — condição em que qualquer listagem/detalhe de negócio deve
// retornar vazio/404, nunca dados de terceiros.
func (s vendedorScope) SemAcesso() bool {
	return s.Restrito && s.VendedorID <= 0
}

// PermiteVendedor reporta se o escopo atual permite acesso a registros do
// vendedorID informado. Sempre true para admin (Restrito=false).
func (s vendedorScope) PermiteVendedor(vendedorID int64) bool {
	if !s.Restrito {
		return true
	}
	return s.VendedorID > 0 && s.VendedorID == vendedorID
}

// resolverVendedorScope calcula o escopo de carteira do usuário autenticado
// a partir do contexto da requisição (userID/role injetados pelo
// JWTMiddleware) e do vínculo id_vendedor persistido em usuarios.
//
// Cada chamada executa sua própria consulta ao banco (sem cache L1),
// garantindo que uma mudança de vínculo id_vendedor reflita imediatamente
// nas próximas requisições, sem depender de reemissão do JWT.
func resolverVendedorScope(ctx context.Context, db *sql.DB) (vendedorScope, error) {
	role, _ := middleware.GetRole(ctx)
	if role == "admin" {
		return vendedorScope{Restrito: false}, nil
	}

	userID, ok := middleware.GetUserID(ctx)
	if !ok {
		// Não deveria ocorrer (a rota exige JWT válido) — trata como o
		// escopo mais restritivo possível.
		return vendedorScope{Restrito: true, VendedorID: 0}, nil
	}

	idVendedor, err := repositories.NewUsuarioRepository().GetIDVendedorByUsuarioID(ctx, db, userID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return vendedorScope{Restrito: true, VendedorID: 0}, nil
		}
		return vendedorScope{}, fmt.Errorf("handlers: resolver escopo de vendedor: %w", err)
	}
	if idVendedor == nil {
		return vendedorScope{Restrito: true, VendedorID: 0}, nil
	}
	return vendedorScope{Restrito: true, VendedorID: *idVendedor}, nil
}

// clienteNaCarteiraDoVendedor reporta se existe vínculo de carteira ativo
// (data_fim IS NULL) entre o vendedor e o cliente informados. Usado para
// impedir que um vendedor com escopo restrito crie/edite registros de
// negócio (oportunidade, visita, ...) associados a um cliente fora da
// própria carteira.
//
// Cada chamada executa sua própria consulta ao banco (sem cache L1).
func clienteNaCarteiraDoVendedor(ctx context.Context, db *sql.DB, vendedorID, clienteID int64) (bool, error) {
	_, err := repositories.NewCarteiraRepository().GetVinculoAtivo(ctx, db, vendedorID, clienteID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("handlers: checar carteira do vendedor: %w", err)
	}
	return true, nil
}
