// Package catalogomysql implementa as portas de catalogoapp sobre MariaDB
// (protocolo MySQL)
// (ARQ-02). O SQL é o do repositório legado, copiado sem alteração; a
// ordenação sai de switch sobre os enums do domínio (C-02) e todo valor vai
// por placeholder (C-03).
package catalogomysql

import (
	"database/sql"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
)

// linha é o que Scan precisa (db.Linha).
type linha = db.Linha

// nullString grava "" como NULL (colunas NULLable, ex.: nota_olfativa).
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// ordemProduto monta " ORDER BY <coluna> <dir>" (padrão "id ASC").
func ordemProduto(o kernel.Ordenacao[catalogo.CampoProduto]) string {
	return " ORDER BY " + colunaProduto(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "ASC")
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
	return " ORDER BY e." + colunaEstoque(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "DESC")
}

// ordemEstoqueRanked monta o ORDER BY da última posição, que lê da tabela
// derivada "ranked" (sem alias): a segunda whitelist do legado.
func ordemEstoqueRanked(o kernel.Ordenacao[catalogo.CampoEstoque]) string {
	return " ORDER BY " + colunaEstoque(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "DESC")
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
