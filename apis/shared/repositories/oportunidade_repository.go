// Package repositories contém funções puras de acesso a dados (stateless).
// Oportunidade queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/vlog"
)

// OportunidadeRepository agrupa queries da tabela oportunidades.
type OportunidadeRepository struct{}

// NewOportunidadeRepository cria um repositório stateless.
func NewOportunidadeRepository() *OportunidadeRepository {
	return &OportunidadeRepository{}
}

// oportunidadeColunas lista as colunas da tabela oportunidades na ordem
// usada pelo scan.
const oportunidadeColunas = `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda, created_at, updated_at`

// OportunidadeFiltro agrupa os filtros opcionais aceitos por List.
// Campos zero-value/vazios são ignorados (não filtram).
type OportunidadeFiltro struct {
	ClienteID       int64
	VendedorID      int64
	Etapa           string
	Origem          string
	DataAberturaDe  string // formato AAAA-MM-DD (inclusive)
	DataAberturaAte string // formato AAAA-MM-DD (inclusive)
	Q               string // busca textual em origem OU etapa (LIKE)
	OrderBy         string // campo de ordenação (whitelist: ver oportunidadeOrderWhitelist); default "oportunidade_id"
	OrderDir        string // "asc" ou "desc" (case-insensitive); default "asc"
}

// oportunidadeOrderWhitelist mapeia os campos de ordenação aceitos pela API
// para as colunas SQL reais da tabela oportunidades.
var oportunidadeOrderWhitelist = map[string]string{
	"id":                "oportunidade_id",
	"data_abertura":     "data_abertura",
	"valor_estimado":    "valor_estimado",
	"probabilidade_pct": "probabilidade_pct",
	"etapa":             "etapa",
	"origem":            "origem",
	"created_at":        "created_at",
	"updated_at":        "updated_at",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "oportunidade_id ASC".
func (f OportunidadeFiltro) orderBy() string {
	return buildOrderByClause(oportunidadeOrderWhitelist, f.OrderBy, f.OrderDir, "oportunidade_id", "ASC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f OportunidadeFiltro) where() (string, []any) {
	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "declarando variável conds")
	var conds []string
	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "declarando variável args")
	var args []any

	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "verificando se f.ClienteID > 0")
	if f.ClienteID > 0 {
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando cláusula de filtro/SQL [cliente_id = ?] em conds")
		conds = append(conds, "cliente_id = ?")
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.ClienteID)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "verificando se f.VendedorID > 0")
	if f.VendedorID > 0 {
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando cláusula de filtro/SQL [vendedor_id = ?] em conds")
		conds = append(conds, "vendedor_id = ?")
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.VendedorID)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "verificando se f.Etapa != \"\"")
	if f.Etapa != "" {
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando cláusula de filtro/SQL [etapa = ?] em conds")
		conds = append(conds, "etapa = ?")
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.Etapa)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "verificando se f.Origem != \"\"")
	if f.Origem != "" {
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando cláusula de filtro/SQL [origem = ?] em conds")
		conds = append(conds, "origem = ?")
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.Origem)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "verificando se f.DataAberturaDe != \"\"")
	if f.DataAberturaDe != "" {
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando cláusula de filtro/SQL [data_abertura >= ?] em conds")
		conds = append(conds, "data_abertura >= ?")
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.DataAberturaDe)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "verificando se f.DataAberturaAte != \"\"")
	if f.DataAberturaAte != "" {
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando cláusula de filtro/SQL [data_abertura <= ?] em conds")
		conds = append(conds, "data_abertura <= ?")
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.DataAberturaAte)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "verificando se f.Q != \"\"")
	if f.Q != "" {
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando cláusula de filtro/SQL [(origem LIKE ? OR etapa LIKE ?)] em conds")
		conds = append(conds, "(origem LIKE ? OR etapa LIKE ?)")
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "definindo like com padrão LIKE do termo de busca (valor omitido)")
		like := "%" + f.Q + "%"
		vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "adicionando 2 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, like, like)
	}

	vlog.Printf("oportunidade_repository.go", "OportunidadeFiltro.where", "verificando se len(conds) == 0")
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna oportunidades paginadas conforme o filtro informado, mais o
// total para meta-dados de paginação.
func (r *OportunidadeRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro OportunidadeFiltro) ([]models.Oportunidade, int, error) {
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "definindo whereClause, args com resultado de chamada a filtro.where")
	whereClause, args := filtro.where()

	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "declarando variável total")
	var total int
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "montando texto da query SQL SELECT em oportunidades em countQ")
	countQ := "SELECT COUNT(*) FROM oportunidades" + whereClause
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query countQ, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count oportunidades: %w", err)
	}

	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "montando texto da query SQL SELECT em q")
	q := "SELECT " + oportunidadeColunas + " FROM oportunidades" + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "adicionando 2 parâmetro(s) de placeholder em queryArgs (valores omitidos)")
	queryArgs := append(append([]any{}, args...), limit, offset)

	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, queryArgs...)
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list oportunidades: %w", err)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "declarando variável out")
	var out []models.Oportunidade
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		o, err := scanOportunidade(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *o)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.List", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list oportunidades iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca uma oportunidade pelo ID. Retorna ErrNotFound se não existir.
func (r *OportunidadeRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Oportunidade, error) {
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.GetByID", "montando texto da query SQL SELECT em q")
	q := "SELECT " + oportunidadeColunas + " FROM oportunidades WHERE oportunidade_id = ? LIMIT 1"
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.GetByID", "definindo row com resultado de execução SQL via db.QueryRowContext (query q, args omitidos)")
	row := db.QueryRowContext(ctx, q, id)
	return scanOportunidade(row)
}

// Create insere uma nova oportunidade e preenche o.OportunidadeID com o id
// gerado nativamente pelo AUTO_INCREMENT do MySQL.
func (r *OportunidadeRepository) Create(ctx context.Context, db *sql.DB, o *models.Oportunidade) error {
	const q = `
		INSERT INTO oportunidades (cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Create", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q,
		o.ClienteID,
		o.VendedorID,
		o.Origem,
		o.DataAbertura,
		o.Etapa,
		o.ProbabilidadePct,
		o.ValorEstimado,
		o.DataFechamento,
		o.CicloDias,
		o.MotivoPerda,
	)
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create oportunidade: %w", err)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Create", "definindo id, err com resultado de chamada a res.LastInsertId")
	id, err := res.LastInsertId()
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create oportunidade lastInsertId: %w", err)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Create", "atribuindo o.OportunidadeID = id")
	o.OportunidadeID = id
	return nil
}

// Update atualiza os campos editáveis de uma oportunidade existente.
// Retorna ErrNotFound se não existir.
func (r *OportunidadeRepository) Update(ctx context.Context, db *sql.DB, id int64, o *models.Oportunidade) error {
	const q = `
		UPDATE oportunidades
		SET cliente_id = ?, vendedor_id = ?, origem = ?, data_abertura = ?, etapa = ?, probabilidade_pct = ?, valor_estimado = ?, data_fechamento = ?, ciclo_dias = ?, motivo_perda = ?
		WHERE oportunidade_id = ?`
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Update", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q,
		o.ClienteID,
		o.VendedorID,
		o.Origem,
		o.DataAbertura,
		o.Etapa,
		o.ProbabilidadePct,
		o.ValorEstimado,
		o.DataFechamento,
		o.CicloDias,
		o.MotivoPerda,
		id,
	)
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update oportunidade: %w", err)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Update", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update oportunidade rowsAffected: %w", err)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Update", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete remove uma oportunidade pela PK oportunidade_id (hard delete — não
// há coluna deleted_at nesta tabela). Retorna ErrNotFound se não existir.
func (r *OportunidadeRepository) Delete(ctx context.Context, db *sql.DB, id int64) error {
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Delete", "definindo res, err com resultado de execução SQL via db.ExecContext (query DELETE em oportunidades, args omitidos)")
	res, err := db.ExecContext(ctx, `DELETE FROM oportunidades WHERE oportunidade_id = ?`, id)
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Delete", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: delete oportunidade: %w", err)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Delete", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Delete", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: delete oportunidade rowsAffected: %w", err)
	}
	vlog.Printf("oportunidade_repository.go", "OportunidadeRepository.Delete", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanOportunidade(s rowScanner) (*models.Oportunidade, error) {
	vlog.Printf("oportunidade_repository.go", "scanOportunidade", "declarando variável o")
	var o models.Oportunidade
	vlog.Printf("oportunidade_repository.go", "scanOportunidade", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
	if err := s.Scan(
		&o.OportunidadeID,
		&o.ClienteID,
		&o.VendedorID,
		&o.Origem,
		&o.DataAbertura,
		&o.Etapa,
		&o.ProbabilidadePct,
		&o.ValorEstimado,
		&o.DataFechamento,
		&o.CicloDias,
		&o.MotivoPerda,
		&o.CreatedAt,
		&o.UpdatedAt,
	); err != nil {
		vlog.Printf("oportunidade_repository.go", "scanOportunidade", "verificando se err == sql.ErrNoRows")
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan oportunidade: %w", err)
	}
	return &o, nil
}
