// Package repositories contém funções puras de acesso a dados (stateless).
// Visita queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/vlog"
)

// VisitaRepository agrupa queries da tabela visitas.
type VisitaRepository struct{}

// NewVisitaRepository cria um repositório stateless.
func NewVisitaRepository() *VisitaRepository {
	return &VisitaRepository{}
}

// visitaColunas lista as colunas da tabela visitas na ordem usada pelo scan.
const visitaColunas = `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min, created_at, updated_at`

// VisitaFiltro agrupa os filtros opcionais aceitos por List.
// Campos zero-value/vazios são ignorados (não filtram).
type VisitaFiltro struct {
	ClienteID     int64
	VendedorID    int64
	Resultado     string
	DataVisitaDe  string // formato AAAA-MM-DD (inclusive)
	DataVisitaAte string // formato AAAA-MM-DD (inclusive)
	Q             string // busca textual em resultado (LIKE)
	OrderBy       string // campo de ordenação (whitelist: ver visitaOrderWhitelist); default "visita_id"
	OrderDir      string // "asc" ou "desc" (case-insensitive); default "asc"
}

// visitaOrderWhitelist mapeia os campos de ordenação aceitos pela API
// para as colunas SQL reais da tabela visitas.
var visitaOrderWhitelist = map[string]string{
	"id":          "visita_id",
	"visita_id":   "visita_id",
	"data_visita": "data_visita",
	"duracao_min": "duracao_min",
	"resultado":   "resultado",
	"created_at":  "created_at",
	"updated_at":  "updated_at",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "visita_id ASC".
func (f VisitaFiltro) orderBy() string {
	return buildOrderByClause(visitaOrderWhitelist, f.OrderBy, f.OrderDir, "visita_id", "ASC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f VisitaFiltro) where() (string, []any) {
	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "declarando variável conds")
	var conds []string
	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "declarando variável args")
	var args []any

	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "verificando se f.ClienteID > 0")
	if f.ClienteID > 0 {
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando cláusula de filtro/SQL [cliente_id = ?] em conds")
		conds = append(conds, "cliente_id = ?")
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.ClienteID)
	}
	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "verificando se f.VendedorID > 0")
	if f.VendedorID > 0 {
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando cláusula de filtro/SQL [vendedor_id = ?] em conds")
		conds = append(conds, "vendedor_id = ?")
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.VendedorID)
	}
	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "verificando se f.Resultado != \"\"")
	if f.Resultado != "" {
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando cláusula de filtro/SQL [resultado = ?] em conds")
		conds = append(conds, "resultado = ?")
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.Resultado)
	}
	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "verificando se f.DataVisitaDe != \"\"")
	if f.DataVisitaDe != "" {
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando cláusula de filtro/SQL [data_visita >= ?] em conds")
		conds = append(conds, "data_visita >= ?")
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.DataVisitaDe)
	}
	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "verificando se f.DataVisitaAte != \"\"")
	if f.DataVisitaAte != "" {
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando cláusula de filtro/SQL [data_visita <= ?] em conds")
		conds = append(conds, "data_visita <= ?")
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.DataVisitaAte)
	}
	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "verificando se f.Q != \"\"")
	if f.Q != "" {
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando cláusula de filtro/SQL [resultado LIKE ?] em conds")
		conds = append(conds, "resultado LIKE ?")
		vlog.Printf("visita_repository.go", "VisitaFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, "%"+f.Q+"%")
	}

	vlog.Printf("visita_repository.go", "VisitaFiltro.where", "verificando se len(conds) == 0")
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna visitas paginadas conforme o filtro informado, mais o total
// para meta-dados de paginação.
func (r *VisitaRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro VisitaFiltro) ([]models.Visita, int, error) {
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("visita_repository.go", "VisitaRepository.List", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("visita_repository.go", "VisitaRepository.List", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("visita_repository.go", "VisitaRepository.List", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	vlog.Printf("visita_repository.go", "VisitaRepository.List", "definindo whereClause, args com resultado de chamada a filtro.where")
	whereClause, args := filtro.where()

	vlog.Printf("visita_repository.go", "VisitaRepository.List", "declarando variável total")
	var total int
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "montando texto da query SQL SELECT em visitas em countQ")
	countQ := "SELECT COUNT(*) FROM visitas" + whereClause
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query countQ, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count visitas: %w", err)
	}

	vlog.Printf("visita_repository.go", "VisitaRepository.List", "montando texto da query SQL SELECT em q")
	q := "SELECT " + visitaColunas + " FROM visitas" + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "adicionando 2 parâmetro(s) de placeholder em queryArgs (valores omitidos)")
	queryArgs := append(append([]any{}, args...), limit, offset)

	vlog.Printf("visita_repository.go", "VisitaRepository.List", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, queryArgs...)
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list visitas: %w", err)
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("visita_repository.go", "VisitaRepository.List", "declarando variável out")
	var out []models.Visita
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		v, err := scanVisita(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *v)
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("visita_repository.go", "VisitaRepository.List", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list visitas iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca uma visita pelo ID. Retorna ErrNotFound se não existir.
func (r *VisitaRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Visita, error) {
	vlog.Printf("visita_repository.go", "VisitaRepository.GetByID", "montando texto da query SQL SELECT em q")
	q := "SELECT " + visitaColunas + " FROM visitas WHERE visita_id = ? LIMIT 1"
	vlog.Printf("visita_repository.go", "VisitaRepository.GetByID", "definindo row com resultado de execução SQL via db.QueryRowContext (query q, args omitidos)")
	row := db.QueryRowContext(ctx, q, id)
	return scanVisita(row)
}

// Create insere uma nova visita e preenche v.VisitaID com o id gerado
// nativamente pelo AUTO_INCREMENT do MySQL.
func (r *VisitaRepository) Create(ctx context.Context, db *sql.DB, v *models.Visita) error {
	const q = `
		INSERT INTO visitas (cliente_id, vendedor_id, data_visita, resultado, duracao_min)
		VALUES (?, ?, ?, ?, ?)`
	vlog.Printf("visita_repository.go", "VisitaRepository.Create", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q,
		v.ClienteID,
		v.VendedorID,
		v.DataVisita,
		v.Resultado,
		v.DuracaoMin,
	)
	vlog.Printf("visita_repository.go", "VisitaRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create visita: %w", err)
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.Create", "definindo id, err com resultado de chamada a res.LastInsertId")
	id, err := res.LastInsertId()
	vlog.Printf("visita_repository.go", "VisitaRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create visita lastInsertId: %w", err)
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.Create", "atribuindo v.VisitaID = id")
	v.VisitaID = id
	return nil
}

// Update atualiza os campos editáveis de uma visita existente.
// Retorna ErrNotFound se não existir.
func (r *VisitaRepository) Update(ctx context.Context, db *sql.DB, id int64, v *models.Visita) error {
	const q = `
		UPDATE visitas
		SET cliente_id = ?, vendedor_id = ?, data_visita = ?, resultado = ?, duracao_min = ?
		WHERE visita_id = ?`
	vlog.Printf("visita_repository.go", "VisitaRepository.Update", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q,
		v.ClienteID,
		v.VendedorID,
		v.DataVisita,
		v.Resultado,
		v.DuracaoMin,
		id,
	)
	vlog.Printf("visita_repository.go", "VisitaRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update visita: %w", err)
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.Update", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("visita_repository.go", "VisitaRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update visita rowsAffected: %w", err)
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.Update", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete remove uma visita pela PK visita_id (hard delete — não há coluna
// deleted_at nesta tabela). Retorna ErrNotFound se não existir.
func (r *VisitaRepository) Delete(ctx context.Context, db *sql.DB, id int64) error {
	vlog.Printf("visita_repository.go", "VisitaRepository.Delete", "definindo res, err com resultado de execução SQL via db.ExecContext (query DELETE em visitas, args omitidos)")
	res, err := db.ExecContext(ctx, `DELETE FROM visitas WHERE visita_id = ?`, id)
	vlog.Printf("visita_repository.go", "VisitaRepository.Delete", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: delete visita: %w", err)
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.Delete", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("visita_repository.go", "VisitaRepository.Delete", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: delete visita rowsAffected: %w", err)
	}
	vlog.Printf("visita_repository.go", "VisitaRepository.Delete", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanVisita(s rowScanner) (*models.Visita, error) {
	vlog.Printf("visita_repository.go", "scanVisita", "declarando variável v")
	var v models.Visita
	vlog.Printf("visita_repository.go", "scanVisita", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
	if err := s.Scan(
		&v.VisitaID,
		&v.ClienteID,
		&v.VendedorID,
		&v.DataVisita,
		&v.Resultado,
		&v.DuracaoMin,
		&v.CreatedAt,
		&v.UpdatedAt,
	); err != nil {
		vlog.Printf("visita_repository.go", "scanVisita", "verificando se err == sql.ErrNoRows")
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan visita: %w", err)
	}
	return &v, nil
}
