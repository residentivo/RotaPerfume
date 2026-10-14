// Package catalogomysql implementa as portas de catalogoapp sobre MySQL
// (ARQ-02). O SQL é o do repositório legado, copiado sem alteração; a
// ordenação sai de switch sobre os enums do domínio (C-02) e todo valor vai
// por placeholder (C-03).
package catalogomysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
)

// linha é o que Scan precisa: *sql.Row e *sql.Rows.
type linha interface {
	Scan(dest ...any) error
}

// consultar executa a listagem e lê todas as linhas com scan. op é o nome
// da operação usado no wrap do erro (nunca valores, O6-iv).
func consultar[T any](ctx context.Context, q db.Querier, op, sqlTexto string, args []any, scan func(linha) (*T, error)) ([]T, error) {
	rows, err := q.QueryContext(ctx, sqlTexto, args...)
	if err != nil {
		return nil, fmt.Errorf("catalogomysql: %s: %w", op, err)
	}
	defer rows.Close()

	var out []T // nil sem linhas: o JSON continua "data": null, como no legado
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalogomysql: %s iteração: %w", op, err)
	}
	return out, nil
}

// paginada acrescenta LIMIT/OFFSET aos args do WHERE (sem alterar o slice
// original, que também vai para o COUNT).
func paginada(args []any, p kernel.Pagina) []any {
	return append(append([]any{}, args...), p.Limite(), p.Offset())
}

// where junta as condições com AND (string vazia sem condições).
func where(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

// nullString grava "" como NULL (colunas NULLable, ex.: nota_olfativa).
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// direcaoSQL traduz a direção; a indefinida (zero value) cai no padrão da
// entidade.
func direcaoSQL(d kernel.Direcao, padrao string) string {
	switch d {
	case kernel.Asc:
		return "ASC"
	case kernel.Desc:
		return "DESC"
	default:
		return padrao
	}
}

// ordemProduto monta " ORDER BY <coluna> <dir>" (padrão "id ASC").
func ordemProduto(o kernel.Ordenacao[catalogo.CampoProduto]) string {
	return " ORDER BY " + colunaProduto(o.Campo()) + " " + direcaoSQL(o.Direcao(), "ASC")
}

// colunaProduto é a whitelist de produtos: enum → coluna. Qualquer valor
// fora do enum cai em "id" (nunca string livre no SQL).
func colunaProduto(c catalogo.CampoProduto) string {
	switch c {
	case catalogo.OrdemProdutoSKU:
		return "sku"
	case catalogo.OrdemProdutoDescricao:
		return "descricao"
	case catalogo.OrdemProdutoCategoria:
		return "categoria"
	case catalogo.OrdemProdutoMarca:
		return "marca"
	case catalogo.OrdemProdutoPrecoTabela:
		return "preco_tabela"
	case catalogo.OrdemProdutoCustoUnitario:
		return "custo_unitario"
	case catalogo.OrdemProdutoDataLancamento:
		return "data_lancamento"
	case catalogo.OrdemProdutoAtivo:
		return "ativo"
	case catalogo.OrdemProdutoCriadoEm:
		return "created_at"
	case catalogo.OrdemProdutoAtualizadoEm:
		return "updated_at"
	default:
		return "id"
	}
}

// ordemEstoque monta o ORDER BY da série histórica, com o alias "e." da
// tabela estoque (as consultas fazem LEFT JOIN com produtos).
func ordemEstoque(o kernel.Ordenacao[catalogo.CampoEstoque]) string {
	return " ORDER BY e." + colunaEstoque(o.Campo()) + " " + direcaoSQL(o.Direcao(), "DESC")
}

// ordemEstoqueRanked monta o ORDER BY da última posição, que lê da tabela
// derivada "ranked" (sem alias): a segunda whitelist do legado.
func ordemEstoqueRanked(o kernel.Ordenacao[catalogo.CampoEstoque]) string {
	return " ORDER BY " + colunaEstoque(o.Campo()) + " " + direcaoSQL(o.Direcao(), "DESC")
}

// colunaEstoque é a whitelist de estoque: enum → coluna (sem alias). Fora do
// enum cai em "data_snapshot".
func colunaEstoque(c catalogo.CampoEstoque) string {
	switch c {
	case catalogo.OrdemEstoqueID:
		return "id"
	case catalogo.OrdemEstoqueSKU:
		return "sku"
	case catalogo.OrdemEstoqueSaldo:
		return "saldo"
	case catalogo.OrdemEstoqueRuptura:
		return "ruptura"
	case catalogo.OrdemEstoqueCriadoEm:
		return "created_at"
	case catalogo.OrdemEstoqueAtualizadoEm:
		return "updated_at"
	default:
		return "data_snapshot"
	}
}
