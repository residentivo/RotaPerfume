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
	"github.com/rotaperfumes/shared/vlog"
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
	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "declarando variável conds")
	var conds []string
	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "declarando variável args")
	var args []any

	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "verificando se f.Status != \"\"")
	if f.Status != "" {
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando cláusula de filtro/SQL [p.status = ?] em conds")
		conds = append(conds, "p.status = ?")
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.Status)
	}
	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "verificando se f.Canal != \"\"")
	if f.Canal != "" {
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando cláusula de filtro/SQL [p.canal = ?] em conds")
		conds = append(conds, "p.canal = ?")
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.Canal)
	}
	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "verificando se f.ClienteID > 0")
	if f.ClienteID > 0 {
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando cláusula de filtro/SQL [p.cliente_id = ?] em conds")
		conds = append(conds, "p.cliente_id = ?")
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.ClienteID)
	}
	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "verificando se f.VendedorID > 0")
	if f.VendedorID > 0 {
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando cláusula de filtro/SQL [p.vendedor_id = ?] em conds")
		conds = append(conds, "p.vendedor_id = ?")
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.VendedorID)
	}
	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "verificando se f.DataInicio != \"\"")
	if f.DataInicio != "" {
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando cláusula de filtro/SQL [p.data_pedido >= ?] em conds")
		conds = append(conds, "p.data_pedido >= ?")
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.DataInicio)
	}
	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "verificando se f.DataFim != \"\"")
	if f.DataFim != "" {
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando cláusula de filtro/SQL [p.data_pedido <= ?] em conds")
		conds = append(conds, "p.data_pedido <= ?")
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.DataFim)
	}
	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "verificando se f.Q != \"\"")
	if f.Q != "" {
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando cláusula de filtro/SQL [c.razao_social LIKE ?] em conds")
		conds = append(conds, "c.razao_social LIKE ?")
		vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, "%"+f.Q+"%")
	}

	vlog.Printf("pedido_repository.go", "PedidoFiltro.where", "verificando se len(conds) == 0")
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna pedidos paginados (sem itens, por performance) conforme o
// filtro informado, mais o total para meta-dados de paginação.
func (r *PedidoRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro PedidoFiltro) ([]PedidoListagem, int, error) {
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("pedido_repository.go", "PedidoRepository.List", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("pedido_repository.go", "PedidoRepository.List", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("pedido_repository.go", "PedidoRepository.List", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "definindo whereClause, args com resultado de chamada a filtro.where")
	whereClause, args := filtro.where()

	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "declarando variável total")
	var total int
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "montando texto da query SQL SELECT em countQ")
	countQ := "SELECT COUNT(*)" + pedidoFrom + whereClause
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query countQ, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count pedidos: %w", err)
	}

	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "montando texto da query SQL SELECT em q")
	q := "SELECT " + pedidoColunas + pedidoFrom + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "adicionando 2 parâmetro(s) de placeholder em queryArgs (valores omitidos)")
	queryArgs := append(append([]any{}, args...), limit, offset)

	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, queryArgs...)
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list pedidos: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "declarando variável out")
	var out []PedidoListagem
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		p, err := scanPedidoListagem(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("pedido_repository.go", "PedidoRepository.List", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list pedidos iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca o cabeçalho de um pedido pelo ID (com nomes de
// cliente/vendedor). Retorna ErrNotFound se não existir.
func (r *PedidoRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*PedidoListagem, error) {
	vlog.Printf("pedido_repository.go", "PedidoRepository.GetByID", "montando texto da query SQL SELECT em q")
	q := "SELECT " + pedidoColunas + pedidoFrom + " WHERE p.pedido_id_origem = ? LIMIT 1"
	vlog.Printf("pedido_repository.go", "PedidoRepository.GetByID", "definindo row com resultado de execução SQL via db.QueryRowContext (query q, args omitidos)")
	row := db.QueryRowContext(ctx, q, id)
	return scanPedidoListagem(row)
}

// ListItensByPedidoID retorna todos os itens de um pedido (com sku/descrição
// do produto), ordenados por item_id_origem.
func (r *PedidoRepository) ListItensByPedidoID(ctx context.Context, db *sql.DB, pedidoID int64) ([]ItemPedidoDetalhe, error) {
	vlog.Printf("pedido_repository.go", "PedidoRepository.ListItensByPedidoID", "montando texto da query SQL SELECT em q")
	q := "SELECT " + itemPedidoColunas + itemPedidoFrom + " WHERE i.pedido_id = ? ORDER BY i.item_id_origem ASC"
	vlog.Printf("pedido_repository.go", "PedidoRepository.ListItensByPedidoID", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, pedidoID)
	vlog.Printf("pedido_repository.go", "PedidoRepository.ListItensByPedidoID", "verificando se err != nil")
	if err != nil {
		return nil, fmt.Errorf("repositories: list itens_pedido: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.ListItensByPedidoID", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("pedido_repository.go", "PedidoRepository.ListItensByPedidoID", "declarando variável out")
	var out []ItemPedidoDetalhe
	vlog.Printf("pedido_repository.go", "PedidoRepository.ListItensByPedidoID", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		it, err := scanItemPedidoDetalhe(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *it)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.ListItensByPedidoID", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("pedido_repository.go", "PedidoRepository.ListItensByPedidoID", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: list itens_pedido iteração: %w", err)
	}
	return out, nil
}

// ExistsByID verifica se existe um pedido com o id informado. Usado pelo
// serviço de Pagamentos para validar pedido_id antes de criar um pagamento.
func (r *PedidoRepository) ExistsByID(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	const q = `SELECT 1 FROM pedidos WHERE pedido_id_origem = ? LIMIT 1`
	vlog.Printf("pedido_repository.go", "PedidoRepository.ExistsByID", "declarando variável one")
	var one int
	vlog.Printf("pedido_repository.go", "PedidoRepository.ExistsByID", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado")
	err := db.QueryRowContext(ctx, q, id).Scan(&one)
	vlog.Printf("pedido_repository.go", "PedidoRepository.ExistsByID", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pedido_repository.go", "PedidoRepository.ExistsByID", "verificando se errors.Is(err, sql.ErrNoRows)")
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
	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "definindo tx, err com resultado de operação de banco via db.BeginTx")
	tx, err := db.BeginTx(ctx, nil)
	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: begin tx create pedido: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "agendando defer de operação de banco via tx.Rollback")
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	const insertPedido = `
		INSERT INTO pedidos (cliente_id, vendedor_id, data_pedido, canal, status, valor_total)
		VALUES (?, ?, ?, ?, ?, ?)`
	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "definindo res, err com resultado de execução SQL via tx.ExecContext (query insertPedido, args omitidos)")
	res, err := tx.ExecContext(ctx, insertPedido,
		p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal,
	)
	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create pedido: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "definindo pedidoID, err com resultado de chamada a res.LastInsertId")
	pedidoID, err := res.LastInsertId()
	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create pedido lastInsertId: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "atribuindo p.PedidoIDOrigem = pedidoID")
	p.PedidoIDOrigem = pedidoID

	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "definindo err com resultado de chamada a insertItensTx e verificando se err != nil")
	if err := insertItensTx(ctx, tx, pedidoID, itens); err != nil {
		return err
	}

	vlog.Printf("pedido_repository.go", "PedidoRepository.CreateComItens", "definindo err com resultado de operação de banco via tx.Commit e verificando se err != nil")
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
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo tx, err com resultado de operação de banco via db.BeginTx")
	tx, err := db.BeginTx(ctx, nil)
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: begin tx update pedido: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "agendando defer de operação de banco via tx.Rollback")
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "declarando variável statusAtual")
	var statusAtual string
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "atribuindo err com resultado de execução SQL via tx.QueryRowContext(...).Scan (query SELECT em pedidos, args omitidos) com leitura do resultado")
	err = tx.QueryRowContext(ctx, `SELECT status FROM pedidos WHERE pedido_id_origem = ? FOR UPDATE`, id).Scan(&statusAtual)
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se errors.Is(err, sql.ErrNoRows)")
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("repositories: lock pedido para update: %w", err)
	}

	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo itensAtuais, err com resultado de chamada a listItensByPedidoIDTx")
	itensAtuais, err := listItensByPedidoIDTx(ctx, tx, id)
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se err != nil")
	if err != nil {
		return err
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo itensAlterados = !itensIguais(itensAtuais, itens)")
	itensAlterados := !itensIguais(itensAtuais, itens)

	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se statusAtual == statusFaturado && itensAlterados")
	if statusAtual == statusFaturado && itensAlterados {
		return ErrPedidoJaFaturadoNaoPodeAlterarItens
	}

	const updatePedido = `
		UPDATE pedidos
		SET cliente_id = ?, vendedor_id = ?, data_pedido = ?, canal = ?, status = ?, valor_total = ?
		WHERE pedido_id_origem = ?`
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo res, err com resultado de execução SQL via tx.ExecContext (query updatePedido, args omitidos)")
	res, err := tx.ExecContext(ctx, updatePedido,
		p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal, id,
	)
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update pedido: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update pedido rowsAffected: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}

	// Pedido já faturado: itens não podem ser reescritos (checado acima),
	// então preserva as linhas de itens_pedido como estão (mesmo
	// item_id_origem/created_at) — apenas o cabeçalho do pedido (ex: status)
	// foi atualizado.
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se statusAtual != statusFaturado")
	if statusAtual != statusFaturado {
		vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo _, err com resultado de execução SQL via tx.ExecContext (query DELETE em itens_pedido, args omitidos) e verificando se err != nil")
		if _, err := tx.ExecContext(ctx, `DELETE FROM itens_pedido WHERE pedido_id = ?`, id); err != nil {
			return fmt.Errorf("repositories: delete itens_pedido: %w", err)
		}

		vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo err com resultado de chamada a insertItensTx e verificando se err != nil")
		if err := insertItensTx(ctx, tx, id, itens); err != nil {
			return err
		}
	}

	// Baixa de estoque automática na transição para "Faturado" (idempotente:
	// só dispara se o pedido NÃO estava faturado antes).
	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "verificando se statusAtual != statusFaturado && p.Status == statusFaturado")
	if statusAtual != statusFaturado && p.Status == statusFaturado {
		vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo estoqueRepo com resultado de chamada a NewEstoqueRepository")
		estoqueRepo := NewEstoqueRepository()
		vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo dataFaturamento com resultado de chamada a time.Now")
		dataFaturamento := time.Now()
		vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "iniciando loop range sobre itens (sem log por iteração)")
		for _, it := range itens {
			sku, err := skuPorProdutoIDTx(ctx, tx, it.ProdutoID)
			if err != nil {
				return err
			}
			if err := estoqueRepo.AjustarSaldoPorFaturamento(ctx, tx, sku, dataFaturamento, it.Quantidade); err != nil {
				return fmt.Errorf("repositories: baixa de estoque no faturamento do pedido %d: %w", id, err)
			}
		}
		vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "loop concluído")
	}

	vlog.Printf("pedido_repository.go", "PedidoRepository.UpdateComItens", "definindo err com resultado de operação de banco via tx.Commit e verificando se err != nil")
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repositories: commit update pedido: %w", err)
	}
	return nil
}

// skuPorProdutoIDTx busca o sku de um produto pelo id, dentro da transação
// informada. Usado pela baixa automática de estoque no faturamento (a tabela
// estoque referencia produtos por sku, não por id).
func skuPorProdutoIDTx(ctx context.Context, tx *sql.Tx, produtoID int64) (string, error) {
	vlog.Printf("pedido_repository.go", "skuPorProdutoIDTx", "declarando variável sku")
	var sku string
	vlog.Printf("pedido_repository.go", "skuPorProdutoIDTx", "definindo err com resultado de execução SQL via tx.QueryRowContext(...).Scan (query SELECT em produtos, args omitidos) com leitura do resultado")
	err := tx.QueryRowContext(ctx, `SELECT sku FROM produtos WHERE id = ? LIMIT 1`, produtoID).Scan(&sku)
	vlog.Printf("pedido_repository.go", "skuPorProdutoIDTx", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pedido_repository.go", "skuPorProdutoIDTx", "verificando se errors.Is(err, sql.ErrNoRows)")
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
	vlog.Printf("pedido_repository.go", "listItensByPedidoIDTx", "definindo rows, err com resultado de execução SQL via tx.QueryContext (query q, args omitidos)")
	rows, err := tx.QueryContext(ctx, q, pedidoID)
	vlog.Printf("pedido_repository.go", "listItensByPedidoIDTx", "verificando se err != nil")
	if err != nil {
		return nil, fmt.Errorf("repositories: list itens_pedido (tx): %w", err)
	}
	vlog.Printf("pedido_repository.go", "listItensByPedidoIDTx", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("pedido_repository.go", "listItensByPedidoIDTx", "declarando variável out")
	var out []models.ItemPedido
	vlog.Printf("pedido_repository.go", "listItensByPedidoIDTx", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var it models.ItemPedido
		if err := rows.Scan(&it.ProdutoID, &it.Quantidade, &it.PrecoPraticado, &it.DescontoPct); err != nil {
			return nil, fmt.Errorf("repositories: scan itens_pedido (tx): %w", err)
		}
		out = append(out, it)
	}
	vlog.Printf("pedido_repository.go", "listItensByPedidoIDTx", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("pedido_repository.go", "listItensByPedidoIDTx", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
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
	vlog.Printf("pedido_repository.go", "itensIguais", "verificando se len(a) != len(b)")
	if len(a) != len(b) {
		return false
	}
	vlog.Printf("pedido_repository.go", "itensIguais", "iniciando loop range sobre a (sem log por iteração)")
	for i := range a {
		if a[i].ProdutoID != b[i].ProdutoID ||
			a[i].Quantidade != b[i].Quantidade ||
			a[i].PrecoPraticado != b[i].PrecoPraticado ||
			a[i].DescontoPct != b[i].DescontoPct {
			return false
		}
	}
	vlog.Printf("pedido_repository.go", "itensIguais", "loop concluído")
	return true
}

// insertItensTx insere os itens de um pedido dentro da transação informada.
// item_id_origem é gerado nativamente pelo AUTO_INCREMENT do MySQL.
func insertItensTx(ctx context.Context, tx *sql.Tx, pedidoID int64, itens []models.ItemPedido) error {
	vlog.Printf("pedido_repository.go", "insertItensTx", "verificando se len(itens) == 0")
	if len(itens) == 0 {
		return nil
	}

	const insertItem = `
		INSERT INTO itens_pedido (pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto)
		VALUES (?, ?, ?, ?, ?, ?)`

	vlog.Printf("pedido_repository.go", "insertItensTx", "iniciando loop range sobre itens (sem log por iteração)")
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
	vlog.Printf("pedido_repository.go", "insertItensTx", "loop concluído")
	return nil
}

// DeleteComItens remove um pedido e seus itens (itens_pedido) em uma única
// transação (hard delete — não há coluna deleted_at nestas tabelas): se a
// exclusão de qualquer uma das tabelas falhar, nada é removido. As regras de
// negócio que bloqueiam a exclusão (pagamentos vinculados, status Faturado)
// são responsabilidade do handler/service chamador — este método assume que
// já foram checadas. Retorna ErrNotFound se o pedido não existir.
func (r *PedidoRepository) DeleteComItens(ctx context.Context, db *sql.DB, id int64) error {
	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "definindo tx, err com resultado de operação de banco via db.BeginTx")
	tx, err := db.BeginTx(ctx, nil)
	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: begin tx delete pedido: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "agendando defer de operação de banco via tx.Rollback")
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "definindo _, err com resultado de execução SQL via tx.ExecContext (query DELETE em itens_pedido, args omitidos) e verificando se err != nil")
	if _, err := tx.ExecContext(ctx, `DELETE FROM itens_pedido WHERE pedido_id = ?`, id); err != nil {
		return fmt.Errorf("repositories: delete itens_pedido: %w", err)
	}

	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "definindo res, err com resultado de execução SQL via tx.ExecContext (query DELETE em pedidos, args omitidos)")
	res, err := tx.ExecContext(ctx, `DELETE FROM pedidos WHERE pedido_id_origem = ?`, id)
	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: delete pedido: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: delete pedido rowsAffected: %w", err)
	}
	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}

	vlog.Printf("pedido_repository.go", "PedidoRepository.DeleteComItens", "definindo err com resultado de operação de banco via tx.Commit e verificando se err != nil")
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repositories: commit delete pedido: %w", err)
	}
	return nil
}

func scanPedidoListagem(s rowScanner) (*PedidoListagem, error) {
	vlog.Printf("pedido_repository.go", "scanPedidoListagem", "declarando variável p")
	var p PedidoListagem
	vlog.Printf("pedido_repository.go", "scanPedidoListagem", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
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
		vlog.Printf("pedido_repository.go", "scanPedidoListagem", "verificando se err == sql.ErrNoRows")
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan pedido: %w", err)
	}
	return &p, nil
}

func scanItemPedidoDetalhe(s rowScanner) (*ItemPedidoDetalhe, error) {
	vlog.Printf("pedido_repository.go", "scanItemPedidoDetalhe", "declarando variável it")
	var it ItemPedidoDetalhe
	vlog.Printf("pedido_repository.go", "scanItemPedidoDetalhe", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
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
		vlog.Printf("pedido_repository.go", "scanItemPedidoDetalhe", "verificando se err == sql.ErrNoRows")
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan item_pedido: %w", err)
	}
	return &it, nil
}
