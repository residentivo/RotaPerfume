package vendasmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqPedidos = "pedidos.go"

// pedidoColunas traz o cabeçalho do pedido + nomes de cliente/vendedor via JOIN.
const pedidoColunas = `p.pedido_id_origem, p.cliente_id, p.vendedor_id, p.data_pedido, p.canal, p.status, p.valor_total, p.created_at, p.updated_at, c.razao_social, v.nome`

// pedidoFrom é o FROM + JOINs comuns às queries de listagem/detalhe de pedidos.
const pedidoFrom = ` FROM pedidos p JOIN clientes c ON c.cliente_id_origem = p.cliente_id JOIN vendedores v ON v.id = p.vendedor_id`

// itemPedidoColunas traz o item de pedido + sku/descrição do produto via JOIN.
const itemPedidoColunas = `i.item_id_origem, i.pedido_id, i.produto_id, i.quantidade, i.preco_praticado, i.desconto_pct, i.valor_bruto, i.created_at, i.updated_at, pr.sku, pr.descricao`

// itemPedidoFrom é o FROM + JOIN comum às queries de itens de pedido.
const itemPedidoFrom = ` FROM itens_pedido i JOIN produtos pr ON pr.id = i.produto_id`

// Pedidos é o repositório de pedidos e itens (portas vendasapp.Pedidos e,
// ligado ao *sql.Tx do UoW, vendasapp.PedidosNaTx).
type Pedidos struct{ q db.Querier }

// NovoPedidos liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoPedidos(q db.Querier) *Pedidos {
	if q == nil {
		panic("vendasmysql: NovoPedidos exige Querier")
	}
	return &Pedidos{q: q}
}

// Listar pagina os pedidos (sem itens) do escopo que casam com o filtro.
func (r *Pedidos) Listar(ctx context.Context, e kernel.Escopo, f vendas.FiltroPedido, p kernel.Pagina) (kernel.Resultado[vendas.Pedido], error) {
	vendedorID, err := vendedorDaListagem(e, f.VendedorID)
	if err != nil {
		return kernel.Resultado[vendas.Pedido]{}, err
	}
	w, args := wherePedido(f, vendedorID)
	countQ := "SELECT COUNT(*)" + pedidoFrom + w
	q := "SELECT " + pedidoColunas + pedidoFrom + w + ordemPedido(f.Ordenacao) + " LIMIT ? OFFSET ?"
	res, err := paginar(ctx, r.q, "pedidos", countQ, q, args, p, scanPedido)
	if err != nil {
		return res, err
	}
	vlog.Printf(arqPedidos, "Pedidos.Listar", "SELECT pedidos linhas=%d total=%d", len(res.Itens), res.Total)
	return res, nil
}

func wherePedido(f vendas.FiltroPedido, vendedorID int64) (string, []any) {
	var conds []string
	var args []any
	if f.Status != "" {
		conds, args = append(conds, "p.status = ?"), append(args, f.Status)
	}
	if f.Canal != "" {
		conds, args = append(conds, "p.canal = ?"), append(args, f.Canal)
	}
	if f.ClienteID > 0 {
		conds, args = append(conds, "p.cliente_id = ?"), append(args, f.ClienteID)
	}
	if vendedorID > 0 {
		conds, args = append(conds, "p.vendedor_id = ?"), append(args, vendedorID)
	}
	if f.DataInicio != "" {
		conds, args = append(conds, "p.data_pedido >= ?"), append(args, f.DataInicio)
	}
	if f.DataFim != "" {
		conds, args = append(conds, "p.data_pedido <= ?"), append(args, f.DataFim)
	}
	if f.Busca != "" {
		conds, args = append(conds, "c.razao_social LIKE ?"), append(args, "%"+f.Busca+"%")
	}
	return db.Where(conds), args
}

// ordemPedido monta " ORDER BY <coluna> <dir>" (padrão
// "p.pedido_id_origem DESC").
func ordemPedido(o kernel.Ordenacao[vendas.CampoPedido]) string {
	return " ORDER BY " + colunaPedido(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "DESC")
}

// colunaPedido é a whitelist de pedidos: enum → coluna (com alias).
func colunaPedido(c vendas.CampoPedido) string {
	switch c {
	case vendas.OrdemPedidoData:
		return "p.data_pedido"
	case vendas.OrdemPedidoCanal:
		return "p.canal"
	case vendas.OrdemPedidoStatus:
		return "p.status"
	case vendas.OrdemPedidoValorTotal:
		return "p.valor_total"
	case vendas.OrdemPedidoCriadoEm:
		return "p.created_at"
	case vendas.OrdemPedidoAtualizadoEm:
		return "p.updated_at"
	case vendas.OrdemPedidoClienteNome:
		return "c.razao_social"
	case vendas.OrdemPedidoVendedorNome:
		return "v.nome"
	default:
		return "p.pedido_id_origem"
	}
}

// PorID busca o cabeçalho; vendas.ErrPedidoNaoEncontrado sem linha. A posse
// (E-04) é do caso de uso, pelo vendedor_id lido aqui.
func (r *Pedidos) PorID(ctx context.Context, id int64) (*vendas.Pedido, error) {
	q := "SELECT " + pedidoColunas + pedidoFrom + " WHERE p.pedido_id_origem = ? LIMIT 1"
	p, err := scanPedido(r.q.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqPedidos, "Pedidos.PorID", "SELECT pedidos linhas=1")
	return p, nil
}

func scanPedido(s db.Linha) (*vendas.Pedido, error) {
	return db.LerUma(s, vendas.ErrPedidoNaoEncontrado, "vendasmysql: scan pedido", func(p *vendas.Pedido) []any {
		return []any{&p.PedidoIDOrigem, &p.ClienteID, &p.VendedorID, &p.DataPedido, &p.Canal, &p.Status,
			&p.ValorTotal, &p.CreatedAt, &p.UpdatedAt, &p.ClienteNome, &p.VendedorNome}
	})
}

// Itens devolve os itens do pedido (com sku/descrição), por item_id_origem.
func (r *Pedidos) Itens(ctx context.Context, pedidoID int64) ([]vendas.ItemPedido, error) {
	q := "SELECT " + itemPedidoColunas + itemPedidoFrom + " WHERE i.pedido_id = ? ORDER BY i.item_id_origem ASC"
	itens, err := db.Consultar(ctx, r.q, "vendasmysql: list itens_pedido", q, []any{pedidoID}, scanItem)
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqPedidos, "Pedidos.Itens", "SELECT itens_pedido linhas=%d", len(itens))
	return itens, nil
}

func scanItem(s db.Linha) (*vendas.ItemPedido, error) {
	return db.LerUma(s, vendas.ErrPedidoNaoEncontrado, "vendasmysql: scan item_pedido", func(it *vendas.ItemPedido) []any {
		return []any{&it.ItemIDOrigem, &it.PedidoID, &it.ProdutoID, &it.Quantidade, &it.PrecoPraticado,
			&it.DescontoPct, &it.ValorBruto, &it.CreatedAt, &it.UpdatedAt, &it.ProdutoSKU, &it.ProdutoDescricao}
	})
}

// Existe informa se o pedido existe (validação do pedido_id do pagamento).
func (r *Pedidos) Existe(ctx context.Context, id int64) (bool, error) {
	return db.Existe(ctx, r.q, arqPedidos, "Pedidos.Existe", "vendasmysql: exists pedido",
		`SELECT 1 FROM pedidos WHERE pedido_id_origem = ? LIMIT 1`, id)
}

// ---------------------------------------------------------------------------
// Escritas (sempre dentro da tx do UoW do pedido)
// ---------------------------------------------------------------------------

// Inserir grava o cabeçalho e os itens; preenche PedidoIDOrigem e os ids
// dos itens. pedido_id_origem e item_id_origem são AUTO_INCREMENT.
func (r *Pedidos) Inserir(ctx context.Context, p *vendas.Pedido) error {
	const insertPedido = `
		INSERT INTO pedidos (cliente_id, vendedor_id, data_pedido, canal, status, valor_total)
		VALUES (?, ?, ?, ?, ?, ?)`
	id, err := db.Inserir(ctx, r.q, "vendasmysql: create pedido", insertPedido,
		p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal)
	if err != nil {
		return err
	}
	p.PedidoIDOrigem = id
	vlog.Printf(arqPedidos, "Pedidos.Inserir", "INSERT pedidos id=%d", id)
	return r.inserirItens(ctx, id, p.Itens)
}

// inserirItens grava os itens no próprio slice (PedidoID e ItemIDOrigem).
func (r *Pedidos) inserirItens(ctx context.Context, pedidoID int64, itens []vendas.ItemPedido) error {
	const insertItem = `
		INSERT INTO itens_pedido (pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto)
		VALUES (?, ?, ?, ?, ?, ?)`
	for i := range itens {
		it := &itens[i]
		it.PedidoID = pedidoID
		id, err := db.Inserir(ctx, r.q, "vendasmysql: create item_pedido", insertItem,
			it.PedidoID, it.ProdutoID, it.Quantidade, it.PrecoPraticado, it.DescontoPct, it.ValorBruto)
		if err != nil {
			return err
		}
		it.ItemIDOrigem = id
	}
	vlog.Printf(arqPedidos, "Pedidos.inserirItens", "INSERT itens_pedido linhas=%d", len(itens))
	return nil
}

// StatusParaAtualizar lê o status atual com SELECT … FOR UPDATE: serializa
// o PUT concorrente do mesmo pedido até o fim da tx.
func (r *Pedidos) StatusParaAtualizar(ctx context.Context, id int64) (string, error) {
	var status string
	err := r.q.QueryRowContext(ctx, `SELECT status FROM pedidos WHERE pedido_id_origem = ? FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", vendas.ErrPedidoNaoEncontrado
	}
	if err != nil {
		return "", fmt.Errorf("vendasmysql: lock pedido para update: %w", err)
	}
	vlog.Printf(arqPedidos, "Pedidos.StatusParaAtualizar", "SELECT pedidos FOR UPDATE linhas=1")
	return status, nil
}

// ItensAtuais lê o conteúdo de negócio dos itens (para comparar com o PUT).
func (r *Pedidos) ItensAtuais(ctx context.Context, id int64) ([]vendas.ItemPedido, error) {
	const q = `SELECT produto_id, quantidade, preco_praticado, desconto_pct FROM itens_pedido WHERE pedido_id = ? ORDER BY item_id_origem ASC`
	itens, err := db.Consultar(ctx, r.q, "vendasmysql: list itens_pedido (tx)", q, []any{id}, func(s db.Linha) (*vendas.ItemPedido, error) {
		var it vendas.ItemPedido
		if err := s.Scan(&it.ProdutoID, &it.Quantidade, &it.PrecoPraticado, &it.DescontoPct); err != nil {
			return nil, fmt.Errorf("vendasmysql: scan itens_pedido (tx): %w", err)
		}
		return &it, nil
	})
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqPedidos, "Pedidos.ItensAtuais", "SELECT itens_pedido linhas=%d", len(itens))
	return itens, nil
}

// AtualizarCabecalho grava o cabeçalho (os valores vêm do domínio, V-03).
func (r *Pedidos) AtualizarCabecalho(ctx context.Context, id int64, p *vendas.Pedido) error {
	const updatePedido = `
		UPDATE pedidos
		SET cliente_id = ?, vendedor_id = ?, data_pedido = ?, canal = ?, status = ?, valor_total = ?
		WHERE pedido_id_origem = ?`
	return db.ExecPorID(ctx, r.q, arqPedidos, "Pedidos.AtualizarCabecalho", "vendasmysql: update pedido", vendas.ErrPedidoNaoEncontrado,
		updatePedido, p.ClienteID, p.VendedorID, p.DataPedido, p.Canal, p.Status, p.ValorTotal, id)
}

// SubstituirItens troca integralmente os itens (DELETE + INSERT).
func (r *Pedidos) SubstituirItens(ctx context.Context, id int64, itens []vendas.ItemPedido) error {
	if _, err := r.q.ExecContext(ctx, `DELETE FROM itens_pedido WHERE pedido_id = ?`, id); err != nil {
		return wrap("delete itens_pedido", err)
	}
	return r.inserirItens(ctx, id, itens)
}

// Excluir remove os itens e o pedido (hard delete: não há deleted_at).
func (r *Pedidos) Excluir(ctx context.Context, id int64) error {
	if _, err := r.q.ExecContext(ctx, `DELETE FROM itens_pedido WHERE pedido_id = ?`, id); err != nil {
		return wrap("delete itens_pedido", err)
	}
	return db.ExecPorID(ctx, r.q, arqPedidos, "Pedidos.Excluir", "vendasmysql: delete pedido", vendas.ErrPedidoNaoEncontrado,
		`DELETE FROM pedidos WHERE pedido_id_origem = ?`, id)
}
