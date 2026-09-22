// Package repositories contém funções puras de acesso a dados (stateless).
// Pedido/ItemPedido queries - cada função executa sua própria query (sem
// cache L1). Create/Update usam transação própria (db.Begin/Commit/Rollback)
// pois envolvem múltiplas tabelas (pedidos + itens_pedido) que precisam ser
// consistentes entre si.
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/models"
)

// PedidoRepository agrupa queries das tabelas pedidos e itens_pedido.
type PedidoRepository struct{}

// NewPedidoRepository cria um repositório stateless.
func NewPedidoRepository() *PedidoRepository {
	return &PedidoRepository{}
}

// statusFaturado é o valor do ENUM status (sql/04_ddl_pedidos.sql) que
// dispara a baixa automática de estoque em UpdateComItens.
const statusFaturado = "Faturado"

// ErrPedidoJaFaturadoNaoPodeAlterarItens é retornado por UpdateComItens
// quando o pedido já está com status "Faturado" e o payload tenta alterar a
// lista de itens/quantidades (permitido apenas mudar o status, ex. para
// "Entregue") — exigência do SecBrain para não dessincronizar itens_pedido
// do estoque já baixado.
var ErrPedidoJaFaturadoNaoPodeAlterarItens = errors.New("pedido já faturado: não é possível alterar os itens, apenas o status")

// PedidoListagem representa um pedido enriquecido com o nome do cliente e do
// vendedor (via JOIN), usado na listagem e no cabeçalho do detalhe.
type PedidoListagem struct {
	models.Pedido
	ClienteNome  string `json:"cliente_nome"`
	VendedorNome string `json:"vendedor_nome"`
}

// ItemPedidoDetalhe representa um item de pedido enriquecido com o sku e a
// descrição do produto (via JOIN), usado no detalhe master-detail do pedido.
type ItemPedidoDetalhe struct {
	models.ItemPedido
	ProdutoSKU       string `json:"produto_sku"`
	ProdutoDescricao string `json:"produto_descricao"`
}

// PedidoDetalhe representa o pedido completo (cabeçalho + itens), retornado
// por GetByID.
type PedidoDetalhe struct {
	PedidoListagem
	Itens []ItemPedidoDetalhe `json:"itens"`
}

// pedidoColunas traz o cabeçalho do pedido + nomes de cliente/vendedor via JOIN.
const pedidoColunas = `p.pedido_id_origem, p.cliente_id, p.vendedor_id, p.data_pedido, p.canal, p.status, p.valor_total, p.created_at, p.updated_at, c.razao_social, v.nome`

// pedidoFrom é o FROM + JOINs comuns às queries de listagem/detalhe de pedidos.
const pedidoFrom = ` FROM pedidos p JOIN clientes c ON c.cliente_id_origem = p.cliente_id JOIN vendedores v ON v.id = p.vendedor_id`

// itemPedidoColunas traz o item de pedido + sku/descrição do produto via JOIN.
const itemPedidoColunas = `i.item_id_origem, i.pedido_id, i.produto_id, i.quantidade, i.preco_praticado, i.desconto_pct, i.valor_bruto, i.created_at, i.updated_at, pr.sku, pr.descricao`

// itemPedidoFrom é o FROM + JOIN comum às queries de itens de pedido.
const itemPedidoFrom = ` FROM itens_pedido i JOIN produtos pr ON pr.id = i.produto_id`

// PedidoFiltro agrupa os filtros opcionais aceitos por List.
// Campos zero-value são ignorados (não filtram).
type PedidoFiltro struct {
	Status     string
	Canal      string
	ClienteID  int64
	VendedorID int64
	DataInicio string // formato AAAA-MM-DD (inclusive)
	DataFim    string // formato AAAA-MM-DD (inclusive)
	Q          string // busca textual na razão social do cliente (LIKE)
	OrderBy    string // campo de ordenação (whitelist: ver pedidoOrderWhitelist); default "id" (mapeado para pedido_id_origem)
	OrderDir   string // "asc" ou "desc" (case-insensitive); default "desc"
}

// pedidoOrderWhitelist mapeia os campos de ordenação aceitos pela API para
// as colunas SQL reais (com alias) da query de listagem de pedidos.
var pedidoOrderWhitelist = map[string]string{
	"id":            "p.pedido_id_origem",
	"data_pedido":   "p.data_pedido",
	"canal":         "p.canal",
	"status":        "p.status",
	"valor_total":   "p.valor_total",
	"created_at":    "p.created_at",
	"updated_at":    "p.updated_at",
	"cliente_nome":  "c.razao_social",
	"vendedor_nome": "v.nome",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "p.pedido_id_origem DESC" (comportamento atual).
func (f PedidoFiltro) orderBy() string {
	return buildOrderByClause(pedidoOrderWhitelist, f.OrderBy, f.OrderDir, "p.pedido_id_origem", "DESC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f PedidoFiltro) where() (string, []any) {
	var conds []string
	var args []any

	if f.Status != "" {
		conds = append(conds, "p.status = ?")
		args = append(args, f.Status)
	}
	if f.Canal != "" {
		conds = append(conds, "p.canal = ?")
		args = append(args, f.Canal)
	}
	if f.ClienteID > 0 {
		conds = append(conds, "p.cliente_id = ?")
		args = append(args, f.ClienteID)
	}
	if f.VendedorID > 0 {
		conds = append(conds, "p.vendedor_id = ?")
		args = append(args, f.VendedorID)
	}
	if f.DataInicio != "" {
		conds = append(conds, "p.data_pedido >= ?")
		args = append(args, f.DataInicio)
	}
	if f.DataFim != "" {
		conds = append(conds, "p.data_pedido <= ?")
		args = append(args, f.DataFim)
	}
	if f.Q != "" {
		conds = append(conds, "c.razao_social LIKE ?")
		args = append(args, "%"+f.Q+"%")
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna pedidos paginados (sem itens, por performance) conforme o
// filtro informado, mais o total para meta-dados de paginação.
func (r *PedidoRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro PedidoFiltro) ([]PedidoListagem, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	whereClause, args := filtro.where()

	var total int
	countQ := "SELECT COUNT(*)" + pedidoFrom + whereClause
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count pedidos: %w", err)
	}

	q := "SELECT " + pedidoColunas + pedidoFrom + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list pedidos: %w", err)
	}
	defer rows.Close()

	var out []PedidoListagem
	for rows.Next() {
		p, err := scanPedidoListagem(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list pedidos iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca o cabeçalho de um pedido pelo ID (com nomes de
// cliente/vendedor). Retorna ErrNotFound se não existir.
func (r *PedidoRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*PedidoListagem, error) {
	q := "SELECT " + pedidoColunas + pedidoFrom + " WHERE p.pedido_id_origem = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, id)
	return scanPedidoListagem(row)
}

// ListItensByPedidoID retorna todos os itens de um pedido (com sku/descrição
// do produto), ordenados por item_id_origem.
func (r *PedidoRepository) ListItensByPedidoID(ctx context.Context, db *sql.DB, pedidoID int64) ([]ItemPedidoDetalhe, error) {
	q := "SELECT " + itemPedidoColunas + itemPedidoFrom + " WHERE i.pedido_id = ? ORDER BY i.item_id_origem ASC"
	rows, err := db.QueryContext(ctx, q, pedidoID)
	if err != nil {
		return nil, fmt.Errorf("repositories: list itens_pedido: %w", err)
	}
	defer rows.Close()

	var out []ItemPedidoDetalhe
	for rows.Next() {
		it, err := scanItemPedidoDetalhe(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: list itens_pedido iteração: %w", err)
	}
	return out, nil
}

// ExistsByID verifica se existe um pedido com o id informado. Usado pelo
// serviço de Pagamentos para validar pedido_id antes de criar um pagamento.
func (r *PedidoRepository) ExistsByID(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	const q = `SELECT 1 FROM pedidos WHERE pedido_id_origem = ? LIMIT 1`
	var one int
	err := db.QueryRowContext(ctx, q, id).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("repositories: exists pedido: %w", err)
	}
	return true, nil
}

// CreateComItens insere um novo pedido e seus itens em uma única transação:
// se qualquer inserção falhar, nada é persistido. pedido_id_origem e
// item_id_origem são gerados nativamente pelo AUTO_INCREMENT do MySQL.
// Preenche p.PedidoIDOrigem e o ItemIDOrigem/PedidoID de cada item em itens.
func (r *PedidoRepository) CreateComItens(ctx context.Context, db *sql.DB, p *models.Pedido, itens []models.ItemPedido) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repositories: begin tx create pedido: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	const insertPedido = `
		INSERT INTO pedidos (cliente_id, vendedor_id, data_pedido, canal, status, valor_total)
		VALUES (?, ?, ?, ?, ?, ?)`
	res, err := tx.ExecContext(ctx, insertPedido,
		p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal,
	)
	if err != nil {
		return fmt.Errorf("repositories: create pedido: %w", err)
	}
	pedidoID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create pedido lastInsertId: %w", err)
	}
	p.PedidoIDOrigem = pedidoID

	if err := insertItensTx(ctx, tx, pedidoID, itens); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repositories: commit create pedido: %w", err)
	}
	return nil
}

// UpdateComItens atualiza o cabeçalho de um pedido existente e substitui
// integralmente a lista de itens (delete + insert), em uma única transação.
// Retorna ErrNotFound se o pedido não existir.
//
// Regras adicionais (exigência do SecBrain 🟣), todas dentro da MESMA
// transação:
//   - Lê o status ATUAL do pedido com SELECT ... FOR UPDATE (serializa
//     concorrência) antes de decidir o que fazer.
//   - Se o pedido já está "Faturado" e o payload tenta alterar os itens
//     (produto/quantidade/preço/desconto), retorna
//     ErrPedidoJaFaturadoNaoPodeAlterarItens sem persistir nada — apenas
//     mudança de status é permitida para pedidos já faturados.
//   - Dispara a baixa de estoque (EstoqueRepository.AjustarSaldoPorFaturamento,
//     dentro da mesma tx) somente na transição status_atual != "Faturado" AND
//     novo_status == "Faturado" — reenvios idempotentes (pedido já faturado
//     recebendo novamente status "Faturado") não baixam estoque de novo. Se a
//     baixa falhar, a transação inteira é revertida (rollback do faturamento).
func (r *PedidoRepository) UpdateComItens(ctx context.Context, db *sql.DB, id int64, p *models.Pedido, itens []models.ItemPedido) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repositories: begin tx update pedido: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	var statusAtual string
	err = tx.QueryRowContext(ctx, `SELECT status FROM pedidos WHERE pedido_id_origem = ? FOR UPDATE`, id).Scan(&statusAtual)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("repositories: lock pedido para update: %w", err)
	}

	itensAtuais, err := listItensByPedidoIDTx(ctx, tx, id)
	if err != nil {
		return err
	}
	itensAlterados := !itensIguais(itensAtuais, itens)

	if statusAtual == statusFaturado && itensAlterados {
		return ErrPedidoJaFaturadoNaoPodeAlterarItens
	}

	const updatePedido = `
		UPDATE pedidos
		SET cliente_id = ?, vendedor_id = ?, data_pedido = ?, canal = ?, status = ?, valor_total = ?
		WHERE pedido_id_origem = ?`
	res, err := tx.ExecContext(ctx, updatePedido,
		p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal, id,
	)
	if err != nil {
		return fmt.Errorf("repositories: update pedido: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update pedido rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}

	// Pedido já faturado: itens não podem ser reescritos (checado acima),
	// então preserva as linhas de itens_pedido como estão (mesmo
	// item_id_origem/created_at) — apenas o cabeçalho do pedido (ex: status)
	// foi atualizado.
	if statusAtual != statusFaturado {
		if _, err := tx.ExecContext(ctx, `DELETE FROM itens_pedido WHERE pedido_id = ?`, id); err != nil {
			return fmt.Errorf("repositories: delete itens_pedido: %w", err)
		}

		if err := insertItensTx(ctx, tx, id, itens); err != nil {
			return err
		}
	}

	// Baixa de estoque automática na transição para "Faturado" (idempotente:
	// só dispara se o pedido NÃO estava faturado antes).
	if statusAtual != statusFaturado && p.Status == statusFaturado {
		estoqueRepo := NewEstoqueRepository()
		dataFaturamento := time.Now()
		for _, it := range itens {
			sku, err := skuPorProdutoIDTx(ctx, tx, it.ProdutoID)
			if err != nil {
				return err
			}
			if err := estoqueRepo.AjustarSaldoPorFaturamento(ctx, tx, sku, dataFaturamento, it.Quantidade); err != nil {
				return fmt.Errorf("repositories: baixa de estoque no faturamento do pedido %d: %w", id, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repositories: commit update pedido: %w", err)
	}
	return nil
}

// skuPorProdutoIDTx busca o sku de um produto pelo id, dentro da transação
// informada. Usado pela baixa automática de estoque no faturamento (a tabela
// estoque referencia produtos por sku, não por id).
func skuPorProdutoIDTx(ctx context.Context, tx *sql.Tx, produtoID int64) (string, error) {
	var sku string
	err := tx.QueryRowContext(ctx, `SELECT sku FROM produtos WHERE id = ? LIMIT 1`, produtoID).Scan(&sku)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("repositories: produto_id %d não encontrado para baixa de estoque", produtoID)
		}
		return "", fmt.Errorf("repositories: sku por produto_id: %w", err)
	}
	return sku, nil
}

// listItensByPedidoIDTx é igual a ListItensByPedidoID, mas roda dentro de
// uma transação (*sql.Tx) — usado por UpdateComItens para comparar os itens
// atuais com o payload recebido antes de decidir se a reescrita de itens é
// permitida.
func listItensByPedidoIDTx(ctx context.Context, tx *sql.Tx, pedidoID int64) ([]models.ItemPedido, error) {
	const q = `SELECT produto_id, quantidade, preco_praticado, desconto_pct FROM itens_pedido WHERE pedido_id = ? ORDER BY item_id_origem ASC`
	rows, err := tx.QueryContext(ctx, q, pedidoID)
	if err != nil {
		return nil, fmt.Errorf("repositories: list itens_pedido (tx): %w", err)
	}
	defer rows.Close()

	var out []models.ItemPedido
	for rows.Next() {
		var it models.ItemPedido
		if err := rows.Scan(&it.ProdutoID, &it.Quantidade, &it.PrecoPraticado, &it.DescontoPct); err != nil {
			return nil, fmt.Errorf("repositories: scan itens_pedido (tx): %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: list itens_pedido (tx) iteração: %w", err)
	}
	return out, nil
}

// itensIguais compara duas listas de itens de pedido por conteúdo de
// negócio (produto_id, quantidade, preco_praticado, desconto_pct), ignorando
// ids/timestamps — usado para detectar se um update de pedido está tentando
// alterar os itens de um pedido já faturado.
func itensIguais(a, b []models.ItemPedido) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ProdutoID != b[i].ProdutoID ||
			a[i].Quantidade != b[i].Quantidade ||
			a[i].PrecoPraticado != b[i].PrecoPraticado ||
			a[i].DescontoPct != b[i].DescontoPct {
			return false
		}
	}
	return true
}

// insertItensTx insere os itens de um pedido dentro da transação informada.
// item_id_origem é gerado nativamente pelo AUTO_INCREMENT do MySQL.
func insertItensTx(ctx context.Context, tx *sql.Tx, pedidoID int64, itens []models.ItemPedido) error {
	if len(itens) == 0 {
		return nil
	}

	const insertItem = `
		INSERT INTO itens_pedido (pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto)
		VALUES (?, ?, ?, ?, ?, ?)`

	for i := range itens {
		it := &itens[i]
		it.PedidoID = pedidoID

		res, err := tx.ExecContext(ctx, insertItem,
			it.PedidoID, it.ProdutoID, it.Quantidade, it.PrecoPraticado, it.DescontoPct, it.ValorBruto,
		)
		if err != nil {
			return fmt.Errorf("repositories: create item_pedido: %w", err)
		}
		itemID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("repositories: create item_pedido lastInsertId: %w", err)
		}
		it.ItemIDOrigem = itemID
	}
	return nil
}

func scanPedidoListagem(s rowScanner) (*PedidoListagem, error) {
	var p PedidoListagem
	if err := s.Scan(
		&p.PedidoIDOrigem,
		&p.ClienteID,
		&p.VendedorID,
		&p.DataPedido,
		&p.Canal,
		&p.Status,
		&p.ValorTotal,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.ClienteNome,
		&p.VendedorNome,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan pedido: %w", err)
	}
	return &p, nil
}

func scanItemPedidoDetalhe(s rowScanner) (*ItemPedidoDetalhe, error) {
	var it ItemPedidoDetalhe
	if err := s.Scan(
		&it.ItemIDOrigem,
		&it.PedidoID,
		&it.ProdutoID,
		&it.Quantidade,
		&it.PrecoPraticado,
		&it.DescontoPct,
		&it.ValorBruto,
		&it.CreatedAt,
		&it.UpdatedAt,
		&it.ProdutoSKU,
		&it.ProdutoDescricao,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan item_pedido: %w", err)
	}
	return &it, nil
}
