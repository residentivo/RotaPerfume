// Package handlers contém handlers HTTP.
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

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

// msgVendedorDesligado é a mensagem exata (contrato SecBrain) devolvida com
// 403 quando o usuário normal está vinculado a um vendedor desligado.
const msgVendedorDesligado = "acesso bloqueado: vendedor desligado"

// errVendedorDesligado é o erro sentinela retornado por resolverVendedorScope
// quando o vendedor vinculado ao usuário está desligado.
var errVendedorDesligado = errors.New("handlers: vendedor desligado")

// resolverVendedorScopeBase calcula o escopo de carteira do usuário
// autenticado a partir do contexto da requisição (userID/role injetados pelo
// JWTMiddleware) e do vínculo id_vendedor persistido em usuarios, SEM checar
// desligamento. Usado diretamente apenas pelo Dashboard, que responde 200
// zerado + vendedor_desligado=true em vez de 403.
//
// Cada chamada executa sua própria consulta ao banco (sem cache L1),
// garantindo que uma mudança de vínculo id_vendedor reflita imediatamente
// nas próximas requisições, sem depender de reemissão do JWT.
func resolverVendedorScopeBase(ctx context.Context, db *sql.DB) (vendedorScope, error) {
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

// resolverVendedorScope calcula o escopo de carteira do usuário autenticado
// (via resolverVendedorScopeBase) e, para usuário normal com vendedor
// vinculado, bloqueia o acesso quando o vendedor está desligado
// (data_desligamento preenchida), retornando errVendedorDesligado.
//
// Regras (contrato 🟣 SecBrain, fail-closed):
//   - admin: sem consulta extra;
//   - vendedor desligado: errVendedorDesligado (403 via responderErroEscopo);
//   - vínculo órfão (vendedor inexistente): escopo "sem vendedor"
//     (Restrito=true, VendedorID=0) — mesmo comportamento de id_vendedor NULL;
//   - qualquer outro erro de banco: erro (500 via responderErroEscopo).
//
// Cada chamada executa suas próprias consultas ao banco (sem cache L1), para
// que um desligamento bloqueie imediatamente as próximas requisições, sem
// depender de reemissão do JWT.
func resolverVendedorScope(r *http.Request, db *sql.DB) (vendedorScope, error) {
	scope, err := resolverVendedorScopeBase(r.Context(), db)
	if err != nil {
		return vendedorScope{}, err
	}
	if !scope.Restrito || scope.VendedorID <= 0 {
		return scope, nil
	}

	desligado, err := repositories.NewVendedorRepository().IsDesligado(r.Context(), db, scope.VendedorID)
	switch {
	case errors.Is(err, repositories.ErrNotFound):
		return vendedorScope{Restrito: true, VendedorID: 0}, nil
	case err != nil:
		return vendedorScope{}, fmt.Errorf("handlers: checar vendedor desligado: %w", err)
	case desligado:
		userID, _ := middleware.GetUserID(r.Context())
		log.Printf("[escopo] bloqueado vendedor_desligado user_id=%d vendedor_id=%d rota=%s",
			userID, scope.VendedorID, r.Method+" "+r.URL.Path)
		return vendedorScope{}, errVendedorDesligado
	}
	return scope, nil
}

// responderErroEscopo traduz o erro de resolverVendedorScope em resposta
// HTTP: errVendedorDesligado → 403 com msgVendedorDesligado; qualquer outro
// erro → log (com a tag do handler) + 500 "erro interno". Fail-closed.
func responderErroEscopo(w http.ResponseWriter, tag string, err error) {
	if errors.Is(err, errVendedorDesligado) {
		writeJSON(w, http.StatusForbidden, nil, msgVendedorDesligado)
		return
	}
	log.Printf("%s escopo: %v", tag, err)
	writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
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
