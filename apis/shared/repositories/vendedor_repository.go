// Package repositories contém funções puras de acesso a dados (stateless).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/vlog"
)

// VendedorRepository agrupa queries da tabela vendedores.
type VendedorRepository struct{}

// NewVendedorRepository cria um repositório stateless.
func NewVendedorRepository() *VendedorRepository {
	return &VendedorRepository{}
}

// vendedorColunas lista as colunas usadas em GetByID/Create/Update.
const vendedorColunas = `id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal, created_at, updated_at`

// VendedorResumo é o formato reduzido de vendedor usado para popular listas
// de seleção (ex: combobox no admin de usuários/pedidos), no mesmo padrão de
// ClienteResumo. Inclui DataDesligamento (nil = ativo) para que o chamador
// possa distinguir vendedores ativos de inativos e, por exemplo, marcá-los
// com um indicador visual — sem essa informação um vendedor inativo ainda
// vinculado a um registro antigo "desaparecia" das opções (bug corrigido
// junto com a remoção do filtro WHERE data_desligamento IS NULL abaixo).
type VendedorResumo struct {
	ID               int64      `json:"id"`
	Nome             string     `json:"nome"`
	Regiao           string     `json:"regiao"`
	UF               string     `json:"uf"`
	DataDesligamento *time.Time `json:"data_desligamento"`
}

// List retorna TODOS os vendedores (ativos e inativos), ordenados por nome.
// DataDesligamento indica o status de cada um (nil = ativo).
//
// Não pagina: é usado para popular listas de seleção (ex: combobox no admin
// de usuários/pedidos). Antes filtrava por data_desligamento IS NULL, mas
// isso fazia vendedores inativos vinculados a registros existentes sumirem
// das opções do formulário — o chamador agora decide como exibir inativos
// (ex: marcador "[inativo]") usando DataDesligamento.
func (r *VendedorRepository) List(ctx context.Context, db *sql.DB) ([]VendedorResumo, error) {
	const q = `
		SELECT id, nome, regiao, uf, data_desligamento
		FROM vendedores
		ORDER BY nome ASC`
	vlog.Printf("vendedor_repository.go", "VendedorRepository.List", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q)
	vlog.Printf("vendedor_repository.go", "VendedorRepository.List", "verificando se err != nil")
	if err != nil {
		return nil, fmt.Errorf("repositories: list vendedores: %w", err)
	}
	return scanVendedoresResumo(rows)
}

// ListResumoByID retorna, no mesmo formato de List, apenas o vendedor com o
// id informado (ativo ou não). Devolve lista vazia (não nula) se não existir.
// Usado para restringir GET /api/vendedores ao próprio vendedor do usuário
// normal — o filtro é aplicado no SQL, nunca em memória.
func (r *VendedorRepository) ListResumoByID(ctx context.Context, db *sql.DB, id int64) ([]VendedorResumo, error) {
	const q = `
		SELECT id, nome, regiao, uf, data_desligamento
		FROM vendedores
		WHERE id = ?
		LIMIT 1`
	vlog.Printf("vendedor_repository.go", "VendedorRepository.ListResumoByID", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, id)
	vlog.Printf("vendedor_repository.go", "VendedorRepository.ListResumoByID", "verificando se err != nil")
	if err != nil {
		return nil, fmt.Errorf("repositories: list vendedor por id: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.ListResumoByID", "definindo out, err com resultado de chamada a scanVendedoresResumo")
	out, err := scanVendedoresResumo(rows)
	vlog.Printf("vendedor_repository.go", "VendedorRepository.ListResumoByID", "verificando se err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.ListResumoByID", "verificando se out == nil")
	if out == nil {
		vlog.Printf("vendedor_repository.go", "VendedorRepository.ListResumoByID", "atribuindo out com literal []VendedorResumo")
		out = []VendedorResumo{}
	}
	return out, nil
}

// scanVendedoresResumo lê as linhas de VendedorResumo e fecha rows.
func scanVendedoresResumo(rows *sql.Rows) ([]VendedorResumo, error) {
	vlog.Printf("vendedor_repository.go", "scanVendedoresResumo", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("vendedor_repository.go", "scanVendedoresResumo", "declarando variável out")
	var out []VendedorResumo
	vlog.Printf("vendedor_repository.go", "scanVendedoresResumo", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var v VendedorResumo
		if err := rows.Scan(&v.ID, &v.Nome, &v.Regiao, &v.UF, &v.DataDesligamento); err != nil {
			return nil, fmt.Errorf("repositories: scan vendedor: %w", err)
		}
		out = append(out, v)
	}
	vlog.Printf("vendedor_repository.go", "scanVendedoresResumo", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("vendedor_repository.go", "scanVendedoresResumo", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: list vendedores iteração: %w", err)
	}
	return out, nil
}

// ExistsByID verifica se existe um vendedor com o id informado (ativo ou não).
func (r *VendedorRepository) ExistsByID(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	const q = `SELECT 1 FROM vendedores WHERE id = ? LIMIT 1`
	vlog.Printf("vendedor_repository.go", "VendedorRepository.ExistsByID", "declarando variável one")
	var one int
	vlog.Printf("vendedor_repository.go", "VendedorRepository.ExistsByID", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado")
	err := db.QueryRowContext(ctx, q, id).Scan(&one)
	vlog.Printf("vendedor_repository.go", "VendedorRepository.ExistsByID", "verificando se err != nil")
	if err != nil {
		vlog.Printf("vendedor_repository.go", "VendedorRepository.ExistsByID", "verificando se errors.Is(err, sql.ErrNoRows)")
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("repositories: exists vendedor: %w", err)
	}
	return true, nil
}

// IsDesligado reporta se o vendedor tem data_desligamento preenchida.
// Retorna ErrNotFound se o vendedor não existir. Cada chamada executa sua
// própria consulta (sem cache L1), para que um desligamento reflita
// imediatamente nas próximas requisições.
func (r *VendedorRepository) IsDesligado(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	const q = `SELECT data_desligamento IS NOT NULL FROM vendedores WHERE id = ? LIMIT 1`
	vlog.Printf("vendedor_repository.go", "VendedorRepository.IsDesligado", "declarando variável desligado")
	var desligado int64
	vlog.Printf("vendedor_repository.go", "VendedorRepository.IsDesligado", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, q, id).Scan(&desligado); err != nil {
		vlog.Printf("vendedor_repository.go", "VendedorRepository.IsDesligado", "verificando se errors.Is(err, sql.ErrNoRows)")
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrNotFound
		}
		return false, fmt.Errorf("repositories: vendedor desligado: %w", err)
	}
	return desligado != 0, nil
}

// GetByID busca um vendedor pelo ID (ativo ou não). Retorna ErrNotFound se não existir.
func (r *VendedorRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Vendedor, error) {
	vlog.Printf("vendedor_repository.go", "VendedorRepository.GetByID", "montando texto da query SQL SELECT em q")
	q := "SELECT " + vendedorColunas + " FROM vendedores WHERE id = ? LIMIT 1"
	vlog.Printf("vendedor_repository.go", "VendedorRepository.GetByID", "definindo row com resultado de execução SQL via db.QueryRowContext (query q, args omitidos)")
	row := db.QueryRowContext(ctx, q, id)
	return scanVendedor(row)
}

// Create insere um novo vendedor e preenche v.ID com o id gerado.
func (r *VendedorRepository) Create(ctx context.Context, db *sql.DB, v *models.Vendedor) error {
	const q = `
		INSERT INTO vendedores (nome, regiao, uf, data_admissao, data_desligamento, meta_mensal)
		VALUES (?, ?, ?, ?, ?, ?)`
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Create", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q,
		v.Nome,
		v.Regiao,
		v.UF,
		v.DataAdmissao,
		v.DataDesligamento,
		v.MetaMensal,
	)
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create vendedor: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Create", "definindo id, err com resultado de chamada a res.LastInsertId")
	id, err := res.LastInsertId()
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create vendedor lastInsertId: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Create", "atribuindo v.ID = id")
	v.ID = id
	return nil
}

// Update atualiza os campos editáveis de um vendedor (data_desligamento não é
// alterado por aqui — ver SetDataDesligamento). Retorna ErrNotFound se não existir.
func (r *VendedorRepository) Update(ctx context.Context, db *sql.DB, id int64, v *models.Vendedor) error {
	const q = `
		UPDATE vendedores
		SET nome = ?, regiao = ?, uf = ?, data_admissao = ?, meta_mensal = ?
		WHERE id = ?`
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Update", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q,
		v.Nome,
		v.Regiao,
		v.UF,
		v.DataAdmissao,
		v.MetaMensal,
		id,
	)
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update vendedor: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Update", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update vendedor rowsAffected: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.Update", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDataDesligamento define (ou limpa, se nil) a data de desligamento de um
// vendedor — soft-delete/reativação. Retorna ErrNotFound se não existir.
// Aceita *sql.DB ou *sql.Tx (ver Execer).
func (r *VendedorRepository) SetDataDesligamento(ctx context.Context, db Execer, id int64, dataDesligamento *sql.NullTime) error {
	const q = `UPDATE vendedores SET data_desligamento = ? WHERE id = ?`
	vlog.Printf("vendedor_repository.go", "VendedorRepository.SetDataDesligamento", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, dataDesligamento, id)
	vlog.Printf("vendedor_repository.go", "VendedorRepository.SetDataDesligamento", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: set data_desligamento vendedor: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.SetDataDesligamento", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("vendedor_repository.go", "VendedorRepository.SetDataDesligamento", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: set data_desligamento vendedor rowsAffected: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.SetDataDesligamento", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarcarDesligamento grava a data de desligamento de um vendedor SEM
// sobrescrever uma data já existente (COALESCE): desligar de novo um vendedor
// já desligado preserva a data original. Idempotente. Retorna ErrNotFound se
// o vendedor não existir — com clientFoundRows=true no DSN, RowsAffected
// conta linhas encontradas, então 0 significa "id inexistente" mesmo quando
// nada muda. Aceita *sql.DB ou *sql.Tx (ver Execer).
//
// Para limpar a data (reativação), use SetDataDesligamento com NullTime
// inválido.
func (r *VendedorRepository) MarcarDesligamento(ctx context.Context, db Execer, id int64, dataDesligamento time.Time) error {
	const q = `UPDATE vendedores SET data_desligamento = COALESCE(data_desligamento, ?) WHERE id = ?`
	vlog.Printf("vendedor_repository.go", "VendedorRepository.MarcarDesligamento", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, dataDesligamento, id)
	vlog.Printf("vendedor_repository.go", "VendedorRepository.MarcarDesligamento", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: marcar desligamento vendedor: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.MarcarDesligamento", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("vendedor_repository.go", "VendedorRepository.MarcarDesligamento", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: marcar desligamento vendedor rowsAffected: %w", err)
	}
	vlog.Printf("vendedor_repository.go", "VendedorRepository.MarcarDesligamento", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanVendedor(s rowScanner) (*models.Vendedor, error) {
	vlog.Printf("vendedor_repository.go", "scanVendedor", "declarando variável v")
	var v models.Vendedor
	vlog.Printf("vendedor_repository.go", "scanVendedor", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
	if err := s.Scan(
		&v.ID,
		&v.Nome,
		&v.Regiao,
		&v.UF,
		&v.DataAdmissao,
		&v.DataDesligamento,
		&v.MetaMensal,
		&v.CreatedAt,
		&v.UpdatedAt,
	); err != nil {
		vlog.Printf("vendedor_repository.go", "scanVendedor", "verificando se err == sql.ErrNoRows")
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan vendedor: %w", err)
	}
	return &v, nil
}
