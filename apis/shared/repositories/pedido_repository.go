// Package repositories contém funções puras de acesso a dados (stateless).
// Pedido/ItemPedido queries - cada função executa sua própria query (sem
// cache L1). Create/Update usam transação própria (db.Begin/Commit/Rollback)
// pois envolvem múltiplas tabelas (pedidos + itens_pedido) que precisam ser
// consistentes entre si.
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
)

// PedidoRepository agrupa queries das tabelas pedidos e itens_pedido.
type PedidoRepository struct{}

// NewPedidoRepository cria um repositório stateless.
func NewPedidoRepository() *PedidoRepository {
	return &PedidoRepository{}
}

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
const pedidoColunas = `p.id, p.pedido_id_origem, p.cliente_id, p.vendedor_id, p.data_pedido, p.canal, p.status, p.valor_total, p.created_at, p.updated_at, c.razao_social, v.nome`

// pedidoFrom é o FROM + JOINs comuns às queries de listagem/detalhe de pedidos.
const pedidoFrom = ` FROM pedidos p JOIN clientes c ON c.id = p.cliente_id JOIN vendedores v ON v.id = p.vendedor_id`

// itemPedidoColunas traz o item de pedido + sku/descrição do produto via JOIN.
const itemPedidoColunas = `i.id, i.item_id_origem, i.pedido_id, i.produto_id, i.quantidade, i.preco_praticado, i.desconto_pct, i.valor_bruto, i.created_at, i.updated_at, pr.sku, pr.descricao`

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

	q := "SELECT " + pedidoColunas + pedidoFrom + whereClause + " ORDER BY p.id DESC LIMIT ? OFFSET ?"
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
	q := "SELECT " + pedidoColunas + pedidoFrom + " WHERE p.id = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, id)
	return scanPedidoListagem(row)
}

// ListItensByPedidoID retorna todos os itens de um pedido (com sku/descrição
// do produto), ordenados por id.
func (r *PedidoRepository) ListItensByPedidoID(ctx context.Context, db *sql.DB, pedidoID int64) ([]ItemPedidoDetalhe, error) {
	q := "SELECT " + itemPedidoColunas + itemPedidoFrom + " WHERE i.pedido_id = ? ORDER BY i.id ASC"
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

// NextPedidoIDOrigem retorna o próximo valor disponível para
// pedido_id_origem (MAX atual + 1). A coluna é UNIQUE e obrigatória, mas não
// é gerada automaticamente pelo banco — pedidos criados via API (fora do CSV
// de origem) recebem um valor sequencial aqui.
func (r *PedidoRepository) NextPedidoIDOrigem(ctx context.Context, db *sql.DB) (int64, error) {
	var next int64
	const q = `SELECT COALESCE(MAX(pedido_id_origem), 0) + 1 FROM pedidos`
	if err := db.QueryRowContext(ctx, q).Scan(&next); err != nil {
		return 0, fmt.Errorf("repositories: next pedido_id_origem: %w", err)
	}
	return next, nil
}

// nextItemIDOrigemTx retorna o próximo valor disponível para item_id_origem
// (MAX atual + 1), consultado dentro da transação para evitar corrida entre
// os itens de um mesmo pedido sendo inseridos em sequência.
func nextItemIDOrigemTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	var next int64
	const q = `SELECT COALESCE(MAX(item_id_origem), 0) + 1 FROM itens_pedido`
	if err := tx.QueryRowContext(ctx, q).Scan(&next); err != nil {
		return 0, fmt.Errorf("repositories: next item_id_origem: %w", err)
	}
	return next, nil
}

// CreateComItens insere um novo pedido e seus itens em uma única transação:
// se qualquer inserção falhar, nada é persistido. Preenche p.ID e o ID de
// cada item em itens.
func (r *PedidoRepository) CreateComItens(ctx context.Context, db *sql.DB, p *models.Pedido, itens []models.ItemPedido) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repositories: begin tx create pedido: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	const insertPedido = `
		INSERT INTO pedidos (pedido_id_origem, cliente_id, vendedor_id, data_pedido, canal, status, valor_total)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := tx.ExecContext(ctx, insertPedido,
		p.PedidoIDOrigem, p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal,
	)
	if err != nil {
		return fmt.Errorf("repositories: create pedido: %w", err)
	}
	pedidoID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create pedido lastInsertId: %w", err)
	}
	p.ID = pedidoID

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
func (r *PedidoRepository) UpdateComItens(ctx context.Context, db *sql.DB, id int64, p *models.Pedido, itens []models.ItemPedido) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repositories: begin tx update pedido: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	const updatePedido = `
		UPDATE pedidos
		SET cliente_id = ?, vendedor_id = ?, data_pedido = ?, canal = ?, status = ?, valor_total = ?
		WHERE id = ?`
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

	if _, err := tx.ExecContext(ctx, `DELETE FROM itens_pedido WHERE pedido_id = ?`, id); err != nil {
		return fmt.Errorf("repositories: delete itens_pedido: %w", err)
	}

	if err := insertItensTx(ctx, tx, id, itens); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repositories: commit update pedido: %w", err)
	}
	return nil
}

// insertItensTx insere os itens de um pedido dentro da transação informada,
// atribuindo item_id_origem sequencialmente e preenchendo o ID de cada item.
func insertItensTx(ctx context.Context, tx *sql.Tx, pedidoID int64, itens []models.ItemPedido) error {
	if len(itens) == 0 {
		return nil
	}

	proximoIDOrigem, err := nextItemIDOrigemTx(ctx, tx)
	if err != nil {
		return err
	}

	const insertItem = `
		INSERT INTO itens_pedido (item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	for i := range itens {
		it := &itens[i]
		it.PedidoID = pedidoID
		it.ItemIDOrigem = proximoIDOrigem
		proximoIDOrigem++

		res, err := tx.ExecContext(ctx, insertItem,
			it.ItemIDOrigem, it.PedidoID, it.ProdutoID, it.Quantidade, it.PrecoPraticado, it.DescontoPct, it.ValorBruto,
		)
		if err != nil {
			return fmt.Errorf("repositories: create item_pedido: %w", err)
		}
		itemID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("repositories: create item_pedido lastInsertId: %w", err)
		}
		it.ID = itemID
	}
	return nil
}

func scanPedidoListagem(s rowScanner) (*PedidoListagem, error) {
	var p PedidoListagem
	if err := s.Scan(
		&p.ID,
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
		&it.ID,
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
