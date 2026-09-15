// Package repositories contém funções puras de acesso a dados (stateless).
// Carteira queries - vínculo histórico Cliente ↔ Vendedor. Cada função
// executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/models"
)

// CarteiraRepository agrupa queries da tabela carteiras.
type CarteiraRepository struct{}

// NewCarteiraRepository cria um repositório stateless.
func NewCarteiraRepository() *CarteiraRepository {
	return &CarteiraRepository{}
}

// ClienteResumo representa um cliente enriquecido com os dados do vínculo de
// carteira (via JOIN), usado na listagem de clientes de um vendedor
// (detalhe master-detail do Vendedor, no mesmo padrão de ItemPedidoDetalhe).
type ClienteResumo struct {
	ID          int64      `json:"id"`
	CNPJ        string     `json:"cnpj"`
	RazaoSocial string     `json:"razao_social"`
	Segmento    string     `json:"segmento"`
	Cidade      string     `json:"cidade"`
	UF          string     `json:"uf"`
	CarteiraID  int64      `json:"carteira_id"`
	DataInicio  time.Time  `json:"data_inicio"`
	DataFim     *time.Time `json:"data_fim"`
}

// clienteResumoColunas traz os dados do cliente + o vínculo de carteira via JOIN.
const clienteResumoColunas = `c.cliente_id_origem, c.cnpj, c.razao_social, c.segmento, c.cidade, c.uf, ca.carteira_id_origem, ca.data_inicio, ca.data_fim`

// clienteResumoFrom é o FROM + JOIN comum às queries de clientes de um vendedor.
const clienteResumoFrom = ` FROM carteiras ca JOIN clientes c ON c.cliente_id_origem = ca.cliente_id`

// ListClientesByVendedorID retorna os clientes vinculados (carteira ativa,
// data_fim IS NULL) a um vendedor, ordenados por razão social.
func (r *CarteiraRepository) ListClientesByVendedorID(ctx context.Context, db *sql.DB, vendedorID int64) ([]ClienteResumo, error) {
	q := "SELECT " + clienteResumoColunas + clienteResumoFrom +
		" WHERE ca.vendedor_id = ? AND ca.data_fim IS NULL ORDER BY c.razao_social ASC"
	rows, err := db.QueryContext(ctx, q, vendedorID)
	if err != nil {
		return nil, fmt.Errorf("repositories: list clientes por vendedor: %w", err)
	}
	defer rows.Close()

	// Inicializado como slice vazio (não nil) para que a serialização JSON
	// produza "clientes": [] em vez de "clientes": null quando o vendedor
	// não tiver nenhum cliente vinculado.
	out := []ClienteResumo{}
	for rows.Next() {
		cr, err := scanClienteResumo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *cr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: list clientes por vendedor iteração: %w", err)
	}
	return out, nil
}

// GetByID busca uma carteira pelo ID. Retorna ErrNotFound se não existir.
func (r *CarteiraRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Carteira, error) {
	const q = `
		SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at
		FROM carteiras
		WHERE carteira_id_origem = ?
		LIMIT 1`
	row := db.QueryRowContext(ctx, q, id)
	return scanCarteira(row)
}

// GetVinculoAtivoByClienteID busca o vínculo de carteira ativo (data_fim IS
// NULL) de um cliente, independentemente do vendedor. Usado para detectar se
// um cliente já está em outra carteira antes de transferi-lo. Retorna
// ErrNotFound se o cliente não tiver vínculo ativo.
func (r *CarteiraRepository) GetVinculoAtivoByClienteID(ctx context.Context, db *sql.DB, clienteID int64) (*models.Carteira, error) {
	const q = `
		SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at
		FROM carteiras
		WHERE cliente_id = ? AND data_fim IS NULL
		LIMIT 1`
	row := db.QueryRowContext(ctx, q, clienteID)
	return scanCarteira(row)
}

// GetVinculoAtivo busca o vínculo de carteira ativo (data_fim IS NULL) entre
// um vendedor e um cliente específicos. Retorna ErrNotFound se não existir.
func (r *CarteiraRepository) GetVinculoAtivo(ctx context.Context, db *sql.DB, vendedorID, clienteID int64) (*models.Carteira, error) {
	const q = `
		SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at
		FROM carteiras
		WHERE vendedor_id = ? AND cliente_id = ? AND data_fim IS NULL
		LIMIT 1`
	row := db.QueryRowContext(ctx, q, vendedorID, clienteID)
	return scanCarteira(row)
}

// GetVinculoByClienteVendedorData busca o vínculo de carteira (ativo ou
// encerrado) entre um cliente e um vendedor com data_inicio exatamente igual
// à data informada. Usado para detectar revinculação no mesmo dia (que
// colidiria com a unique key cliente_id+vendedor_id+data_inicio ao tentar
// criar um novo registro). Retorna ErrNotFound se não existir.
func (r *CarteiraRepository) GetVinculoByClienteVendedorData(ctx context.Context, db *sql.DB, clienteID, vendedorID int64, dataInicio time.Time) (*models.Carteira, error) {
	const q = `
		SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at
		FROM carteiras
		WHERE cliente_id = ? AND vendedor_id = ? AND data_inicio = ?
		LIMIT 1`
	row := db.QueryRowContext(ctx, q, clienteID, vendedorID, dataInicio.Format("2006-01-02"))
	return scanCarteira(row)
}

// ReativarVinculo reabre um vínculo de carteira previamente encerrado (SET
// data_fim = NULL), usado ao revincular um cliente ao mesmo vendedor no
// mesmo dia em que o vínculo foi encerrado (evita duplicar a linha, o que
// violaria a unique key cliente_id+vendedor_id+data_inicio). Retorna
// ErrNotFound se o vínculo não existir.
func (r *CarteiraRepository) ReativarVinculo(ctx context.Context, db *sql.DB, id int64) error {
	const q = `UPDATE carteiras SET data_fim = NULL WHERE carteira_id_origem = ?`
	res, err := db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("repositories: reativar vinculo carteira: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: reativar vinculo carteira rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Create insere um novo vínculo de carteira e preenche c.CarteiraIDOrigem
// com o id gerado nativamente pelo AUTO_INCREMENT do MySQL.
func (r *CarteiraRepository) Create(ctx context.Context, db *sql.DB, c *models.Carteira) error {
	const q = `
		INSERT INTO carteiras (cliente_id, vendedor_id, data_inicio, data_fim)
		VALUES (?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q,
		c.ClienteID,
		c.VendedorID,
		c.DataInicio,
		c.DataFim,
	)
	if err != nil {
		return fmt.Errorf("repositories: create carteira: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create carteira lastInsertId: %w", err)
	}
	c.CarteiraIDOrigem = id
	return nil
}

// EncerrarVinculo marca o fim de um vínculo ativo (SET data_fim), usado ao
// trocar o vendedor de um cliente sem apagar o histórico. Retorna
// ErrNotFound se o vínculo não existir.
func (r *CarteiraRepository) EncerrarVinculo(ctx context.Context, db *sql.DB, id int64, dataFim time.Time) error {
	const q = `UPDATE carteiras SET data_fim = ? WHERE carteira_id_origem = ?`
	res, err := db.ExecContext(ctx, q, dataFim, id)
	if err != nil {
		return fmt.Errorf("repositories: encerrar vinculo carteira: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: encerrar vinculo carteira rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete remove um vínculo de carteira pelo ID. Retorna ErrNotFound se não existir.
func (r *CarteiraRepository) Delete(ctx context.Context, db *sql.DB, id int64) error {
	const q = `DELETE FROM carteiras WHERE carteira_id_origem = ?`
	res, err := db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("repositories: delete carteira: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: delete carteira rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCarteira(s rowScanner) (*models.Carteira, error) {
	var c models.Carteira
	if err := s.Scan(
		&c.CarteiraIDOrigem,
		&c.ClienteID,
		&c.VendedorID,
		&c.DataInicio,
		&c.DataFim,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan carteira: %w", err)
	}
	return &c, nil
}

func scanClienteResumo(s rowScanner) (*ClienteResumo, error) {
	var cr ClienteResumo
	if err := s.Scan(
		&cr.ID, // ClienteResumo.ID mapeia c.cliente_id_origem (identidade do cliente)
		&cr.CNPJ,
		&cr.RazaoSocial,
		&cr.Segmento,
		&cr.Cidade,
		&cr.UF,
		&cr.CarteiraID,
		&cr.DataInicio,
		&cr.DataFim,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan cliente resumo: %w", err)
	}
	return &cr, nil
}
