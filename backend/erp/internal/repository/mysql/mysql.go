package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"backend/erp/internal/config"
	"backend/erp/internal/models"
	"backend/erp/internal/repository"
)

// Compile-time interface checks
var (
	_ repository.ProdutoRepository   = (*ProdutoRepository)(nil)
	_ repository.PedidoRepository    = (*PedidoRepository)(nil)
	_ repository.ItemPedidoRepository = (*ItemPedidoRepository)(nil)
	_ repository.PagamentoRepository = (*PagamentoRepository)(nil)
	_ repository.EstoqueRepository   = (*EstoqueRepository)(nil)
	_ repository.UsuarioRepository   = (*UsuarioRepository)(nil)
	_ repository.DashboardRepository = (*DashboardRepository)(nil)
)

// ========== ProdutoRepository ==========

type ProdutoRepository struct {
	db *sql.DB
}

func NewProdutoRepository() *ProdutoRepository {
	return &ProdutoRepository{db: config.DB}
}

func (r *ProdutoRepository) ListarTodos(ctx context.Context) ([]models.Produto, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT sku, COALESCE(descricao,''), COALESCE(categoria,''), COALESCE(marca,''), COALESCE(nota_olfativa,''), COALESCE(preco_tabela,0), COALESCE(custo_unitario,0), COALESCE(unidade,''), COALESCE(ativo,'S'), data_lancamento FROM produtos ORDER BY sku")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var produtos []models.Produto
	for rows.Next() {
		var p models.Produto
		if err := rows.Scan(&p.SKU, &p.Descricao, &p.Categoria, &p.Marca, &p.NotaOlfativa, &p.PrecoTabela, &p.CustoUnitario, &p.Unidade, &p.Ativo, &p.DataLancamento); err != nil {
			return nil, err
		}
		produtos = append(produtos, p)
	}
	return produtos, rows.Err()
}

func (r *ProdutoRepository) BuscarPorSKU(ctx context.Context, sku string) (*models.Produto, error) {
	var p models.Produto
	err := r.db.QueryRowContext(ctx, "SELECT sku, COALESCE(descricao,''), COALESCE(categoria,''), COALESCE(marca,''), COALESCE(nota_olfativa,''), COALESCE(preco_tabela,0), COALESCE(custo_unitario,0), COALESCE(unidade,''), COALESCE(ativo,'S'), data_lancamento FROM produtos WHERE sku = ?", sku).Scan(&p.SKU, &p.Descricao, &p.Categoria, &p.Marca, &p.NotaOlfativa, &p.PrecoTabela, &p.CustoUnitario, &p.Unidade, &p.Ativo, &p.DataLancamento)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *ProdutoRepository) Criar(ctx context.Context, p models.Produto) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO produtos (sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, ativo) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", p.SKU, p.Descricao, p.Categoria, p.Marca, p.NotaOlfativa, p.PrecoTabela, p.CustoUnitario, p.Unidade, p.Ativo)
	return err
}

func (r *ProdutoRepository) Atualizar(ctx context.Context, sku string, p models.Produto) error {
	_, err := r.db.ExecContext(ctx, "UPDATE produtos SET descricao=?, categoria=?, marca=?, nota_olfativa=?, preco_tabela=?, custo_unitario=?, unidade=?, ativo=? WHERE sku=?", p.Descricao, p.Categoria, p.Marca, p.NotaOlfativa, p.PrecoTabela, p.CustoUnitario, p.Unidade, p.Ativo, sku)
	return err
}

// ========== PedidoRepository ==========

type PedidoRepository struct {
	db *sql.DB
}

func NewPedidoRepository() *PedidoRepository {
	return &PedidoRepository{db: config.DB}
}

func (r *PedidoRepository) ListarTodos(ctx context.Context) ([]models.Pedido, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, cliente_id, vendedor_id, COALESCE(data_pedido,NOW()), COALESCE(canal,''), COALESCE(status,''), COALESCE(valor_total,0) FROM pedidos ORDER BY id DESC LIMIT 500")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pedidos []models.Pedido
	for rows.Next() {
		var p models.Pedido
		if err := rows.Scan(&p.ID, &p.ClienteID, &p.VendedorID, &p.DataPedido, &p.Canal, &p.Status, &p.ValorTotal); err != nil {
			return nil, err
		}
		pedidos = append(pedidos, p)
	}
	return pedidos, rows.Err()
}

func (r *PedidoRepository) BuscarPorID(ctx context.Context, id int64) (*models.Pedido, error) {
	var p models.Pedido
	err := r.db.QueryRowContext(ctx, "SELECT id, cliente_id, vendedor_id, COALESCE(data_pedido,NOW()), COALESCE(canal,''), COALESCE(status,''), COALESCE(valor_total,0) FROM pedidos WHERE id = ?", id).Scan(&p.ID, &p.ClienteID, &p.VendedorID, &p.DataPedido, &p.Canal, &p.Status, &p.ValorTotal)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PedidoRepository) Criar(ctx context.Context, p models.Pedido, itens []models.ItemPedido) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Calcula valor total
	total := 0.0
	for _, it := range itens {
		bruto := float64(it.Quantidade) * it.PrecoPraticado * (1 - it.DescontoPct/100)
		total += bruto
	}

	res, err := tx.ExecContext(ctx, "INSERT INTO pedidos (cliente_id, vendedor_id, data_pedido, canal, status, valor_total) VALUES (?, ?, NOW(), ?, ?, ?)", p.ClienteID, p.VendedorID, p.Canal, p.Status, total)
	if err != nil {
		return 0, err
	}
	pedidoID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Inserir itens
	for _, it := range itens {
		bruto := float64(it.Quantidade) * it.PrecoPraticado * (1 - it.DescontoPct/100)
		_, err = tx.ExecContext(ctx, "INSERT INTO itens_pedido (pedido_id, sku, quantidade, preco_praticado, desconto_pct, valor_bruto) VALUES (?, ?, ?, ?, ?, ?)",
			pedidoID, it.SKU, it.Quantidade, it.PrecoPraticado, it.DescontoPct, bruto)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return pedidoID, nil
}

// ========== ItemPedidoRepository ==========

type ItemPedidoRepository struct {
	db *sql.DB
}

func NewItemPedidoRepository() *ItemPedidoRepository {
	return &ItemPedidoRepository{db: config.DB}
}

func (r *ItemPedidoRepository) ListarPorPedido(ctx context.Context, pedidoID int64) ([]models.ItemPedido, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, pedido_id, sku, COALESCE(quantidade,0), COALESCE(preco_praticado,0), COALESCE(desconto_pct,0), COALESCE(valor_bruto,0) FROM itens_pedido WHERE pedido_id = ?", pedidoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var itens []models.ItemPedido
	for rows.Next() {
		var it models.ItemPedido
		if err := rows.Scan(&it.ID, &it.PedidoID, &it.SKU, &it.Quantidade, &it.PrecoPraticado, &it.DescontoPct, &it.ValorBruto); err != nil {
			return nil, err
		}
		itens = append(itens, it)
	}
	return itens, rows.Err()
}

// ========== PagamentoRepository ==========

type PagamentoRepository struct {
	db *sql.DB
}

func NewPagamentoRepository() *PagamentoRepository {
	return &PagamentoRepository{db: config.DB}
}

func (r *PagamentoRepository) ListarTodos(ctx context.Context) ([]models.Pagamento, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, pedido_id, COALESCE(forma_pagamento,''), COALESCE(parcelas,1), COALESCE(valor,0), COALESCE(taxa_pct,0), COALESCE(valor_liquido,0), COALESCE(data_vencimento,NOW()), data_pagamento, COALESCE(status_pagamento,'') FROM pagamentos ORDER BY id DESC LIMIT 500")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pagamentos []models.Pagamento
	for rows.Next() {
		var p models.Pagamento
		if err := rows.Scan(&p.ID, &p.PedidoID, &p.FormaPagamento, &p.Parcelas, &p.Valor, &p.TaxaPct, &p.ValorLiquido, &p.DataVencimento, &p.DataPagamento, &p.StatusPagamento); err != nil {
			return nil, err
		}
		pagamentos = append(pagamentos, p)
	}
	return pagamentos, rows.Err()
}

func (r *PagamentoRepository) BuscarPorID(ctx context.Context, id int64) (*models.Pagamento, error) {
	var p models.Pagamento
	err := r.db.QueryRowContext(ctx, "SELECT id, pedido_id, COALESCE(forma_pagamento,''), COALESCE(parcelas,1), COALESCE(valor,0), COALESCE(taxa_pct,0), COALESCE(valor_liquido,0), COALESCE(data_vencimento,NOW()), data_pagamento, COALESCE(status_pagamento,'') FROM pagamentos WHERE id = ?", id).Scan(&p.ID, &p.PedidoID, &p.FormaPagamento, &p.Parcelas, &p.Valor, &p.TaxaPct, &p.ValorLiquido, &p.DataVencimento, &p.DataPagamento, &p.StatusPagamento)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PagamentoRepository) Criar(ctx context.Context, p models.Pagamento) (int64, error) {
	var dataPag interface{}
	if p.DataPagamento.Valid {
		dataPag = p.DataPagamento.Time
	} else {
		dataPag = nil
	}
	res, err := r.db.ExecContext(ctx, "INSERT INTO pagamentos (pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		p.PedidoID, p.FormaPagamento, p.Parcelas, p.Valor, p.TaxaPct, p.ValorLiquido, p.DataVencimento, dataPag, p.StatusPagamento)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *PagamentoRepository) Atualizar(ctx context.Context, id int64, p models.Pagamento) error {
	var dataPag interface{}
	if p.DataPagamento.Valid {
		dataPag = p.DataPagamento.Time
	} else {
		dataPag = nil
	}
	_, err := r.db.ExecContext(ctx, "UPDATE pagamentos SET forma_pagamento=?, parcelas=?, valor=?, taxa_pct=?, valor_liquido=?, data_vencimento=?, data_pagamento=?, status_pagamento=? WHERE id=?",
		p.FormaPagamento, p.Parcelas, p.Valor, p.TaxaPct, p.ValorLiquido, p.DataVencimento, dataPag, p.StatusPagamento, id)
	return err
}

// ========== EstoqueRepository ==========

type EstoqueRepository struct {
	db *sql.DB
}

func NewEstoqueRepository() *EstoqueRepository {
	return &EstoqueRepository{db: config.DB}
}

func (r *EstoqueRepository) ListarPorSKU(ctx context.Context, sku string) ([]models.Estoque, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT data_snapshot, sku, COALESCE(saldo,0), COALESCE(ruptura,'N') FROM estoque WHERE sku = ? ORDER BY data_snapshot DESC", sku)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var estoques []models.Estoque
	for rows.Next() {
		var e models.Estoque
		if err := rows.Scan(&e.DataSnapshot, &e.SKU, &e.Saldo, &e.Ruptura); err != nil {
			return nil, err
		}
		estoques = append(estoques, e)
	}
	return estoques, rows.Err()
}

func (r *EstoqueRepository) UltimoSnapshot(ctx context.Context, sku string) (*models.Estoque, error) {
	var e models.Estoque
	err := r.db.QueryRowContext(ctx, "SELECT data_snapshot, sku, COALESCE(saldo,0), COALESCE(ruptura,'N') FROM estoque WHERE sku = ? ORDER BY data_snapshot DESC LIMIT 1", sku).Scan(&e.DataSnapshot, &e.SKU, &e.Saldo, &e.Ruptura)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *EstoqueRepository) ListarEmRuptura(ctx context.Context) ([]models.DashboardRuptura, error) {
	// Pega último snapshot por SKU
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.sku, COALESCE(p.descricao,''), COALESCE(e.saldo,0)
		FROM estoque e
		LEFT JOIN produtos p ON p.sku = e.sku
		WHERE e.data_snapshot = (SELECT MAX(data_snapshot) FROM estoque)
		  AND e.ruptura = 'S'
		ORDER BY e.sku
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rupturas []models.DashboardRuptura
	for rows.Next() {
		var r models.DashboardRuptura
		if err := rows.Scan(&r.SKU, &r.Descricao, &r.Saldo); err != nil {
			return nil, err
		}
		rupturas = append(rupturas, r)
	}
	return rupturas, rows.Err()
}

// ========== UsuarioRepository ==========

type UsuarioRepository struct {
	db *sql.DB
}

func NewUsuarioRepository() *UsuarioRepository {
	return &UsuarioRepository{db: config.DB}
}

func (r *UsuarioRepository) BuscarPorLogin(ctx context.Context, login string) (*models.Usuario, error) {
	query := "SELECT id, login, email, senha_hash, tipo_usuario, vendedor_id, gerenciado_por, ativo FROM usuarios WHERE login = ? LIMIT 1"
	var u models.Usuario
	err := r.db.QueryRowContext(ctx, query, login).Scan(&u.ID, &u.Login, &u.Email, &u.SenhaHash, &u.TipoUsuario, &u.VendedorID, &u.GerenciadoPor, &u.Ativo)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ========== DashboardRepository ==========

type DashboardRepository struct {
	db *sql.DB
}

func NewDashboardRepository() *DashboardRepository {
	return &DashboardRepository{db: config.DB}
}

func (r *DashboardRepository) ObterTotaisPorFormaPagamento(ctx context.Context) ([]models.DashboardFinanceiroTotais, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(forma_pagamento,'N/I') AS forma,
		       COALESCE(SUM(valor),0) AS total,
		       COALESCE(SUM(valor_liquido),0) AS liquido,
		       COUNT(*) AS qtd
		FROM pagamentos
		GROUP BY forma_pagamento
		ORDER BY total DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totais []models.DashboardFinanceiroTotais
	for rows.Next() {
		var t models.DashboardFinanceiroTotais
		if err := rows.Scan(&t.FormaPagamento, &t.TotalValor, &t.TotalLiquido, &t.Quantidade); err != nil {
			return nil, err
		}
		totais = append(totais, t)
	}
	return totais, rows.Err()
}

// helper
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("data vazia")
	}
	return time.Parse("2006-01-02", s)
}
