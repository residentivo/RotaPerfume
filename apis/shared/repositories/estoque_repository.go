// Package repositories contém funções puras de acesso a dados (stateless).
// Estoque queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/models"
)

// EstoqueRepository agrupa queries da tabela estoque.
type EstoqueRepository struct{}

// NewEstoqueRepository cria um repositório stateless.
func NewEstoqueRepository() *EstoqueRepository {
	return &EstoqueRepository{}
}

// estoqueColunas seleciona os campos de estoque mais a descrição do produto
// (via LEFT JOIN com produtos), usado por List/UltimaPosicaoPorSku/GetByID.
// LEFT JOIN para não esconder um snapshot cujo sku não exista mais em
// produtos (não deveria acontecer, dada a FK, mas evita que uma
// inconsistência de dados faça o registro sumir da listagem).
const estoqueColunas = `e.id, e.data_snapshot, e.sku, COALESCE(p.descricao, '') AS produto_descricao, e.saldo, e.ruptura, e.created_at, e.updated_at`
const estoqueFrom = ` FROM estoque e LEFT JOIN produtos p ON p.sku = e.sku`

// estoqueDataLayout é o formato usado para comparar/gravar data_snapshot.
const estoqueDataLayout = "2006-01-02"

// EstoqueFiltro agrupa os filtros opcionais aceitos por List e
// UltimaPosicaoPorSku. Campos vazios/nil são ignorados (não filtram).
type EstoqueFiltro struct {
	SKU      string
	DataDe   *time.Time
	DataAte  *time.Time
	Ruptura  *bool
	OrderBy  string // campo de ordenação (whitelist: ver estoqueOrderWhitelist); default "data_snapshot"
	OrderDir string // "asc" ou "desc" (case-insensitive); default "desc"
}

// estoqueOrderWhitelist mapeia os campos de ordenação aceitos pela API para
// as colunas SQL reais (qualificadas com o alias `e` da tabela estoque, já
// que as queries sempre fazem LEFT JOIN com produtos). Usado para sanitizar
// ORDER BY dinâmico (proteção contra SQL injection via order_by).
var estoqueOrderWhitelist = map[string]string{
	"id":            "e.id",
	"data_snapshot": "e.data_snapshot",
	"sku":           "e.sku",
	"saldo":         "e.saldo",
	"ruptura":       "e.ruptura",
	"created_at":    "e.created_at",
	"updated_at":    "e.updated_at",
}

// estoqueOrderWhitelistRanked é a variante usada para ordenar o resultado
// FINAL de UltimaPosicaoPorSku, que seleciona a partir de uma tabela
// derivada ("ranked") sem alias `e`/`p` — os nomes de coluna aí são os
// aliases definidos em estoqueColunas (id, data_snapshot, sku, saldo, ...).
var estoqueOrderWhitelistRanked = map[string]string{
	"id":            "id",
	"data_snapshot": "data_snapshot",
	"sku":           "sku",
	"saldo":         "saldo",
	"ruptura":       "ruptura",
	"created_at":    "created_at",
	"updated_at":    "updated_at",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "data_snapshot DESC" (última posição primeiro).
func (f EstoqueFiltro) orderBy() string {
	return buildOrderByClause(estoqueOrderWhitelist, f.OrderBy, f.OrderDir, "e.data_snapshot", "DESC")
}

// orderByRanked é o equivalente a orderBy() para uso sobre a tabela derivada
// "ranked" de UltimaPosicaoPorSku (ver estoqueOrderWhitelistRanked).
func (f EstoqueFiltro) orderByRanked() string {
	return buildOrderByClause(estoqueOrderWhitelistRanked, f.OrderBy, f.OrderDir, "data_snapshot", "DESC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args
// correspondentes, com colunas qualificadas pelo alias `e` (tabela estoque),
// já que todas as queries que a utilizam fazem LEFT JOIN com produtos.
// Retorna string vazia quando não há filtros.
func (f EstoqueFiltro) where() (string, []any) {
	var conds []string
	var args []any

	if f.SKU != "" {
		conds = append(conds, "e.sku = ?")
		args = append(args, f.SKU)
	}
	if f.DataDe != nil {
		conds = append(conds, "e.data_snapshot >= ?")
		args = append(args, f.DataDe.Format(estoqueDataLayout))
	}
	if f.DataAte != nil {
		conds = append(conds, "e.data_snapshot <= ?")
		args = append(args, f.DataAte.Format(estoqueDataLayout))
	}
	if f.Ruptura != nil {
		conds = append(conds, "e.ruptura = ?")
		args = append(args, *f.Ruptura)
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna registros de estoque paginados conforme o filtro informado,
// mais o total para meta-dados de paginação.
func (r *EstoqueRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro EstoqueFiltro) ([]models.Estoque, int, error) {
	page, limit = normalizePagination(page, limit)
	offset := (page - 1) * limit

	whereClause, args := filtro.where()

	var total int
	countQ := "SELECT COUNT(*)" + estoqueFrom + whereClause
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count estoque: %w", err)
	}

	q := "SELECT " + estoqueColunas + estoqueFrom + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list estoque: %w", err)
	}
	defer rows.Close()

	var out []models.Estoque
	for rows.Next() {
		e, err := scanEstoque(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list estoque iteração: %w", err)
	}
	return out, total, nil
}

// UltimaPosicaoPorSku retorna, para cada sku (respeitando o filtro), apenas
// o registro de data_snapshot mais recente — usado pela listagem padrão do
// frontend, que sempre mostra a última posição de estoque por produto. Com
// filtro.DataAte informado, retorna a última posição de cada sku até
// (inclusive) aquela data — histórico "como estava naquele dia".
//
// Implementado com window function ROW_NUMBER() OVER (PARTITION BY sku
// ORDER BY data_snapshot DESC), suportada desde MySQL 8.0 (dialeto usado
// no projeto).
func (r *EstoqueRepository) UltimaPosicaoPorSku(ctx context.Context, db *sql.DB, page, limit int, filtro EstoqueFiltro) ([]models.Estoque, int, error) {
	page, limit = normalizePagination(page, limit)
	offset := (page - 1) * limit

	whereClause, args := filtro.where()

	const rankedCTE = `
		SELECT ` + estoqueColunas + `,
			ROW_NUMBER() OVER (PARTITION BY e.sku ORDER BY e.data_snapshot DESC, e.id DESC) AS rn
		` + estoqueFrom

	var total int
	countQ := "SELECT COUNT(*) FROM (" + rankedCTE + whereClause + ") ranked WHERE rn = 1"
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count estoque última posição: %w", err)
	}

	q := "SELECT id, data_snapshot, sku, produto_descricao, saldo, ruptura, created_at, updated_at FROM (" + rankedCTE + whereClause + ") ranked WHERE rn = 1" + filtro.orderByRanked() + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list estoque última posição: %w", err)
	}
	defer rows.Close()

	var out []models.Estoque
	for rows.Next() {
		e, err := scanEstoque(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list estoque última posição iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca um registro de estoque pelo ID. Retorna ErrNotFound se não existir.
func (r *EstoqueRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Estoque, error) {
	q := "SELECT " + estoqueColunas + estoqueFrom + " WHERE e.id = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, id)
	return scanEstoque(row)
}

// Create insere um novo registro de estoque e preenche e.ID com o id gerado.
func (r *EstoqueRepository) Create(ctx context.Context, db *sql.DB, e *models.Estoque) error {
	const q = `
		INSERT INTO estoque (data_snapshot, sku, saldo, ruptura)
		VALUES (?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q,
		e.DataSnapshot.Format(estoqueDataLayout),
		e.SKU,
		e.Saldo,
		e.Ruptura,
	)
	if err != nil {
		return fmt.Errorf("repositories: create estoque: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create estoque lastInsertId: %w", err)
	}
	e.ID = id
	return nil
}

// Update atualiza os campos editáveis de um registro de estoque (data_snapshot
// e sku não são alterados por aqui — para mudar a chave de negócio, crie um
// novo registro). Retorna ErrNotFound se não existir.
func (r *EstoqueRepository) Update(ctx context.Context, db *sql.DB, id int64, e *models.Estoque) error {
	const q = `
		UPDATE estoque
		SET saldo = ?, ruptura = ?
		WHERE id = ?`
	res, err := db.ExecContext(ctx, q,
		e.Saldo,
		e.Ruptura,
		id,
	)
	if err != nil {
		return fmt.Errorf("repositories: update estoque: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update estoque rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpsertPorDataSku faz upsert do saldo/ruptura ABSOLUTO de um sku numa data:
// se já existir registro para aquele par (data_snapshot, sku) — UNIQUE KEY
// uk_estoque_data_sku —, sobrescreve saldo/ruptura; senão cria um novo
// registro.
//
// Uso: importador de CSV (dados/erp/estoque.csv traz saldo absoluto do
// ERP). NÃO usar para baixa de estoque por faturamento de pedidos — nesse
// caso, use AjustarSaldoPorFaturamento (delta relativo, seguro sob
// concorrência e dentro da mesma transação do faturamento).
func (r *EstoqueRepository) UpsertPorDataSku(ctx context.Context, db *sql.DB, sku string, data time.Time, saldo int, ruptura bool) error {
	const q = `
		INSERT INTO estoque (data_snapshot, sku, saldo, ruptura)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			saldo = VALUES(saldo),
			ruptura = VALUES(ruptura)`
	_, err := db.ExecContext(ctx, q, data.Format(estoqueDataLayout), sku, saldo, ruptura)
	if err != nil {
		return fmt.Errorf("repositories: upsert estoque por data/sku: %w", err)
	}
	return nil
}

// AjustarSaldoPorFaturamento aplica um DELTA relativo ao saldo de um sku
// numa data, para uso EXCLUSIVO do faturamento de pedidos (Backend). Ao
// contrário de UpsertPorDataSku (que grava o saldo ABSOLUTO vindo do CSV do
// ERP e por isso não é seguro para baixa concorrente), este método faz
// `saldo = saldo - delta` atomicamente no próprio SQL, evitando race
// condition de leitura-then-escrita sob concorrência (dois pedidos
// faturando o mesmo sku ao mesmo tempo).
//
// Recebe uma *sql.Tx (não *sql.DB) porque o Backend DEVE chamar este método
// dentro da MESMA transação de PedidoRepository.UpdateComItens, garantindo
// atomicidade entre faturar o pedido e baixar o estoque (exigência do
// SecBrain 🟣).
//
// Se ainda não existir registro para (data_snapshot, sku) — ex: primeiro
// movimento do dia, antes do import do CSV rodar — o INSERT inicial parte
// de saldo = 0 e já aplica o delta (saldo final = -delta se for uma baixa;
// para dar entrada de saldo positivo, delta pode ser negativo). Chamadores
// que precisem partir do saldo do dia anterior devem resolver esse valor
// antes de chamar (ex: consultar a última posição via
// EstoqueRepository.UltimaPosicaoPorSku) — este método não faz esse
// carry-over sozinho, para manter a query atômica e simples.
//
// ruptura é recalculada automaticamente: true se o saldo resultante for <= 0.
//
// Usa SELECT ... FOR UPDATE para serializar concorrência entre faturamentos
// simultâneos do mesmo (data_snapshot, sku) antes do upsert, já que o
// próprio INSERT ... ON DUPLICATE KEY UPDATE sozinho não impede leituras
// fantasmas em isolamentos menos estritos.
func (r *EstoqueRepository) AjustarSaldoPorFaturamento(ctx context.Context, tx *sql.Tx, sku string, data time.Time, delta int) error {
	dataStr := data.Format(estoqueDataLayout)

	// Serializa concorrência: bloqueia a linha (se existir) antes do upsert,
	// para que dois faturamentos simultâneos do mesmo sku/data não pisem um
	// no outro entre o SELECT e o INSERT/UPDATE.
	var existing int
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM estoque WHERE data_snapshot = ? AND sku = ? FOR UPDATE`,
		dataStr, sku,
	).Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("repositories: lock estoque para ajuste de faturamento: %w", err)
	}

	// VALUES(saldo) carrega a CONTRIBUIÇÃO (não o saldo final): -delta.
	// - Caminho INSERT (linha nova): o valor literal inserido em `saldo` É
	//   a contribuição, então o saldo final já nasce correto: 0 -> -delta.
	// - Caminho UPDATE (linha existente): `saldo = saldo + VALUES(saldo)`
	//   soma a contribuição ao saldo atual: existing + (-delta) = existing - delta.
	// Isso evita o erro comum de usar "saldo - VALUES(saldo)" (que inverteria
	// o sinal no caminho UPDATE, já que VALUES(saldo) teria que representar
	// tanto o saldo final do INSERT quanto o delta do UPDATE simultaneamente
	// — impossível com uma única expressão de sinal fixo).
	const q = `
		INSERT INTO estoque (data_snapshot, sku, saldo, ruptura)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			saldo = saldo + VALUES(saldo),
			ruptura = (saldo + VALUES(saldo)) <= 0`
	contribuicao := -delta
	_, err = tx.ExecContext(ctx, q, dataStr, sku, contribuicao, contribuicao <= 0)
	if err != nil {
		return fmt.Errorf("repositories: ajustar saldo estoque por faturamento: %w", err)
	}
	return nil
}

// normalizePagination aplica os limites padrão de paginação do projeto
// (page mínimo 1, limit entre 1 e 100, default 20).
func normalizePagination(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func scanEstoque(s rowScanner) (*models.Estoque, error) {
	var e models.Estoque
	var dataSnapshot time.Time
	if err := s.Scan(
		&e.ID,
		&dataSnapshot,
		&e.SKU,
		&e.ProdutoDescricao,
		&e.Saldo,
		&e.Ruptura,
		&e.CreatedAt,
		&e.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan estoque: %w", err)
	}
	e.DataSnapshot = dataSnapshot
	return &e, nil
}
