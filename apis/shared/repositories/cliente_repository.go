// Package repositories contém funções puras de acesso a dados (stateless).
// Cliente queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/vlog"
)

// ClienteRepository agrupa queries da tabela clientes.
type ClienteRepository struct{}

// NewClienteRepository cria um repositório stateless.
func NewClienteRepository() *ClienteRepository {
	return &ClienteRepository{}
}

// clienteColunas usa COALESCE em bairro pois a coluna é NULLable no banco,
// mas o model.Cliente.Bairro é string (não ponteiro) — NULL vira "".
const clienteColunas = `cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, COALESCE(bairro, ''), data_cadastro, ativo, created_at, updated_at`

// clienteNaCarteiraCond restringe clientes à carteira ativa (data_fim IS
// NULL) de um vendedor. O vendedor_id vai sempre por placeholder.
const clienteNaCarteiraCond = "cliente_id_origem IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = ? AND data_fim IS NULL)"

// carteiraWhere monta a cláusula " WHERE ..." (ou string vazia) combinando
// com AND as condições fixas informadas e, quando vendedorID > 0, o filtro
// de carteira ativa. vendedorID <= 0 (admin) não adiciona restrição. As
// condições fixas são sempre constantes do código (nunca entrada do
// usuário); o vendedor_id vai exclusivamente por placeholder.
func carteiraWhere(vendedorID int64, conds ...string) (string, []any) {
	vlog.Printf("cliente_repository.go", "carteiraWhere", "declarando variável args")
	var args []any
	vlog.Printf("cliente_repository.go", "carteiraWhere", "verificando se vendedorID > 0")
	if vendedorID > 0 {
		vlog.Printf("cliente_repository.go", "carteiraWhere", "adicionando cláusula SQL (clienteNaCarteiraCond) em conds")
		conds = append(conds, clienteNaCarteiraCond)
		vlog.Printf("cliente_repository.go", "carteiraWhere", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, vendedorID)
	}
	vlog.Printf("cliente_repository.go", "carteiraWhere", "verificando se len(conds) == 0")
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// ClienteFiltro agrupa os filtros opcionais aceitos por List.
// Campos vazios/nil são ignorados (não filtram).
type ClienteFiltro struct {
	UF         string
	Segmento   string
	Ativo      *bool
	Q          string // busca textual em razao_social OU cnpj (LIKE)
	QCNPJ      string // termo da busca em cnpj quando difere de Q (ex.: Q com máscara normalizado); vazio = usa Q. Só vale com Q preenchido
	VendedorID int64  // > 0 restringe aos clientes na carteira ativa desse vendedor (ver tabela carteiras)
	OrderBy    string // campo de ordenação (whitelist: ver clienteOrderWhitelist); default "cliente_id_origem"
	OrderDir   string // "asc" ou "desc" (case-insensitive); default "asc"
}

// clienteOrderWhitelist mapeia os campos de ordenação aceitos pela API para
// as colunas SQL reais da tabela clientes.
var clienteOrderWhitelist = map[string]string{
	"id":            "cliente_id_origem",
	"razao_social":  "razao_social",
	"cnpj":          "cnpj",
	"segmento":      "segmento",
	"cidade":        "cidade",
	"uf":            "uf",
	"data_cadastro": "data_cadastro",
	"ativo":         "ativo",
	"created_at":    "created_at",
	"updated_at":    "updated_at",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "cliente_id_origem ASC" (comportamento atual).
func (f ClienteFiltro) orderBy() string {
	return buildOrderByClause(clienteOrderWhitelist, f.OrderBy, f.OrderDir, "cliente_id_origem", "ASC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f ClienteFiltro) where() (string, []any) {
	vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "declarando variável conds")
	var conds []string
	vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "declarando variável args")
	var args []any

	vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "verificando se f.UF != \"\"")
	if f.UF != "" {
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando cláusula de filtro/SQL [uf = ?] em conds")
		conds = append(conds, "uf = ?")
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.UF)
	}
	vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "verificando se f.Segmento != \"\"")
	if f.Segmento != "" {
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando cláusula de filtro/SQL [segmento = ?] em conds")
		conds = append(conds, "segmento = ?")
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.Segmento)
	}
	vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "verificando se f.Ativo != nil")
	if f.Ativo != nil {
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando cláusula de filtro/SQL [ativo = ?] em conds")
		conds = append(conds, "ativo = ?")
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, *f.Ativo)
	}
	vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "verificando se f.Q != \"\"")
	if f.Q != "" {
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando cláusula de filtro/SQL [(razao_social LIKE ? OR cnpj LIKE ?)] em conds")
		conds = append(conds, "(razao_social LIKE ? OR cnpj LIKE ?)")
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "definindo termoCNPJ = f.QCNPJ")
		termoCNPJ := f.QCNPJ
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "verificando se termoCNPJ == \"\"")
		if termoCNPJ == "" {
			vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "atribuindo termoCNPJ = f.Q")
			termoCNPJ = f.Q
		}
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando 2 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, "%"+f.Q+"%", "%"+termoCNPJ+"%")
	}
	vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "verificando se f.VendedorID > 0")
	if f.VendedorID > 0 {
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando cláusula SQL (clienteNaCarteiraCond) em conds")
		conds = append(conds, clienteNaCarteiraCond)
		vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.VendedorID)
	}

	vlog.Printf("cliente_repository.go", "ClienteFiltro.where", "verificando se len(conds) == 0")
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna clientes paginados conforme o filtro informado, mais o total
// para meta-dados de paginação.
func (r *ClienteRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro ClienteFiltro) ([]models.Cliente, int, error) {
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("cliente_repository.go", "ClienteRepository.List", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("cliente_repository.go", "ClienteRepository.List", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("cliente_repository.go", "ClienteRepository.List", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "definindo whereClause, args com resultado de chamada a filtro.where")
	whereClause, args := filtro.where()

	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "declarando variável total")
	var total int
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "montando texto da query SQL SELECT em clientes em countQ")
	countQ := "SELECT COUNT(*) FROM clientes" + whereClause
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query countQ, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count clientes: %w", err)
	}

	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "montando texto da query SQL SELECT em q")
	q := "SELECT " + clienteColunas + " FROM clientes" + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "adicionando 2 parâmetro(s) de placeholder em queryArgs (valores omitidos)")
	queryArgs := append(append([]any{}, args...), limit, offset)

	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, queryArgs...)
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list clientes: %w", err)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "declarando variável out")
	var out []models.Cliente
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		c, err := scanCliente(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("cliente_repository.go", "ClienteRepository.List", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list clientes iteração: %w", err)
	}
	return out, total, nil
}

// ExistsByID verifica se existe um cliente com o id informado (ativo ou não).
func (r *ClienteRepository) ExistsByID(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	const q = `SELECT 1 FROM clientes WHERE cliente_id_origem = ? LIMIT 1`
	vlog.Printf("cliente_repository.go", "ClienteRepository.ExistsByID", "declarando variável one")
	var one int
	vlog.Printf("cliente_repository.go", "ClienteRepository.ExistsByID", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado")
	err := db.QueryRowContext(ctx, q, id).Scan(&one)
	vlog.Printf("cliente_repository.go", "ClienteRepository.ExistsByID", "verificando se err != nil")
	if err != nil {
		vlog.Printf("cliente_repository.go", "ClienteRepository.ExistsByID", "verificando se errors.Is(err, sql.ErrNoRows)")
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("repositories: exists cliente: %w", err)
	}
	return true, nil
}

// GetByID busca um cliente pelo ID. Retorna ErrNotFound se não existir.
func (r *ClienteRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Cliente, error) {
	vlog.Printf("cliente_repository.go", "ClienteRepository.GetByID", "montando texto da query SQL SELECT em q")
	q := "SELECT " + clienteColunas + " FROM clientes WHERE cliente_id_origem = ? LIMIT 1"
	vlog.Printf("cliente_repository.go", "ClienteRepository.GetByID", "definindo row com resultado de execução SQL via db.QueryRowContext (query q, args omitidos)")
	row := db.QueryRowContext(ctx, q, id)
	return scanCliente(row)
}

// SetAtivo ativa/inativa um cliente. Retorna ErrNotFound se não existir.
func (r *ClienteRepository) SetAtivo(ctx context.Context, db *sql.DB, id int64, ativo bool) error {
	const q = `UPDATE clientes SET ativo = ? WHERE cliente_id_origem = ?`
	vlog.Printf("cliente_repository.go", "ClienteRepository.SetAtivo", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, ativo, id)
	vlog.Printf("cliente_repository.go", "ClienteRepository.SetAtivo", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: set ativo cliente: %w", err)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.SetAtivo", "definindo n, _ com resultado de chamada a res.RowsAffected")
	n, _ := res.RowsAffected()
	vlog.Printf("cliente_repository.go", "ClienteRepository.SetAtivo", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountTotal retorna o total de clientes cadastrados. vendedorID > 0
// restringe à carteira ativa do vendedor; 0 (admin) = todos.
func (r *ClienteRepository) CountTotal(ctx context.Context, db *sql.DB, vendedorID int64) (int, error) {
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountTotal", "definindo where, args com resultado de chamada a carteiraWhere")
	where, args := carteiraWhere(vendedorID)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountTotal", "declarando variável total")
	var total int
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountTotal", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query dinâmica, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM clientes"+where, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("repositories: count total clientes: %w", err)
	}
	return total, nil
}

// CountPorAtivo retorna o total de clientes com o status ativo informado.
// vendedorID > 0 restringe à carteira ativa do vendedor; 0 (admin) = todos.
func (r *ClienteRepository) CountPorAtivo(ctx context.Context, db *sql.DB, ativo bool, vendedorID int64) (int, error) {
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorAtivo", "definindo where, carteiraArgs com resultado de chamada a carteiraWhere")
	where, carteiraArgs := carteiraWhere(vendedorID, "ativo = ?")
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorAtivo", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
	args := append([]any{ativo}, carteiraArgs...)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorAtivo", "declarando variável total")
	var total int
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorAtivo", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query dinâmica, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM clientes"+where, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("repositories: count clientes por ativo: %w", err)
	}
	return total, nil
}

// CountNovosNoPeriodo retorna o total de clientes cujo data_cadastro caiu no
// período informado. periodo: "today" (dia atual), "week" (ultimos 7 dias,
// incluindo hoje) ou "month" (mês atual). vendedorID > 0 restringe à
// carteira ativa do vendedor; 0 (admin) = todos.
func (r *ClienteRepository) CountNovosNoPeriodo(ctx context.Context, db *sql.DB, periodo string, vendedorID int64) (int, error) {
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "declarando variável whereClause")
	var whereClause string
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "avaliando switch sobre periodo")
	switch periodo {
	case "today":
		vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "atribuindo whereClause = \"data_cadastro = CURDATE()\"")
		whereClause = "data_cadastro = CURDATE()"
	case "week":
		vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "atribuindo whereClause = \"data_cadastro >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)\"")
		whereClause = "data_cadastro >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)"
	default:
		vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "atribuindo whereClause = \"YEAR(data_cadastro) = YEAR(CURDATE()) AND MONTH(data_cadastro) = MONTH(CURDATE())\"")
		whereClause = "YEAR(data_cadastro) = YEAR(CURDATE()) AND MONTH(data_cadastro) = MONTH(CURDATE())"
	}

	vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "definindo where, args com resultado de chamada a carteiraWhere")
	where, args := carteiraWhere(vendedorID, whereClause)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "montando texto da query SQL SELECT em clientes em q")
	q := "SELECT COUNT(*) FROM clientes" + where
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "declarando variável total")
	var total int
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountNovosNoPeriodo", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("repositories: count novos clientes: %w", err)
	}
	return total, nil
}

// SegmentoContagem representa a contagem de clientes de um segmento.
type SegmentoContagem struct {
	Segmento string `json:"segmento"`
	Total    int    `json:"total"`
}

// CountPorSegmento retorna a distribuição de clientes por segmento,
// ordenada do maior para o menor total. vendedorID > 0 restringe à carteira
// ativa do vendedor; 0 (admin) = todos. Sem linhas, devolve slice vazio
// (nunca nil), serializado como [].
func (r *ClienteRepository) CountPorSegmento(ctx context.Context, db *sql.DB, vendedorID int64) ([]SegmentoContagem, error) {
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "definindo where, args com resultado de chamada a carteiraWhere")
	where, args := carteiraWhere(vendedorID)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "montando texto da query SQL SELECT em clientes em q")
	q := `
		SELECT segmento, COUNT(*) AS total
		FROM clientes` + where + `
		GROUP BY segmento
		ORDER BY total DESC`

	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, args...)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "verificando se err != nil")
	if err != nil {
		return nil, fmt.Errorf("repositories: count por segmento: %w", err)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "definindo out com resultado de chamada a make")
	out := make([]SegmentoContagem, 0)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var sc SegmentoContagem
		if err := rows.Scan(&sc.Segmento, &sc.Total); err != nil {
			return nil, fmt.Errorf("repositories: scan segmento: %w", err)
		}
		out = append(out, sc)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorSegmento", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: count por segmento iteração: %w", err)
	}
	return out, nil
}

// UFContagem representa a contagem de clientes de uma UF.
type UFContagem struct {
	UF    string `json:"uf"`
	Total int    `json:"total"`
}

// CountPorUF retorna a distribuição de clientes por UF, ordenada do maior
// para o menor total. vendedorID > 0 restringe à carteira ativa do vendedor;
// 0 (admin) = todos. Sem linhas, devolve slice vazio (nunca nil).
func (r *ClienteRepository) CountPorUF(ctx context.Context, db *sql.DB, vendedorID int64) ([]UFContagem, error) {
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "definindo where, args com resultado de chamada a carteiraWhere")
	where, args := carteiraWhere(vendedorID)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "montando texto da query SQL SELECT em clientes em q")
	q := `
		SELECT uf, COUNT(*) AS total
		FROM clientes` + where + `
		GROUP BY uf
		ORDER BY total DESC`

	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, args...)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "verificando se err != nil")
	if err != nil {
		return nil, fmt.Errorf("repositories: count por uf: %w", err)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "definindo out com resultado de chamada a make")
	out := make([]UFContagem, 0)
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var uc UFContagem
		if err := rows.Scan(&uc.UF, &uc.Total); err != nil {
			return nil, fmt.Errorf("repositories: scan uf: %w", err)
		}
		out = append(out, uc)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("cliente_repository.go", "ClienteRepository.CountPorUF", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: count por uf iteração: %w", err)
	}
	return out, nil
}

// Create insere um novo cliente e preenche c.ClienteIDOrigem com o id
// gerado nativamente pelo AUTO_INCREMENT do MySQL.
// Aceita *sql.DB ou *sql.Tx (ver Execer): o cadastro de cliente por usuário
// normal grava cliente + carteira na mesma transação (SEC-01).
func (r *ClienteRepository) Create(ctx context.Context, db Execer, c *models.Cliente) error {
	const q = `
		INSERT INTO clientes (cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	vlog.Printf("cliente_repository.go", "ClienteRepository.Create", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q,
		c.CNPJ,
		c.RazaoSocial,
		c.Segmento,
		c.Cidade,
		c.UF,
		c.Bairro,
		c.DataCadastro,
		c.Ativo,
	)
	vlog.Printf("cliente_repository.go", "ClienteRepository.Create", "verificando se err != nil")
	if err != nil {
		vlog.Printf("cliente_repository.go", "ClienteRepository.Create", "verificando se isDuplicateKey(err, uqClientesCNPJ)")
		if isDuplicateKey(err, uqClientesCNPJ) {
			return fmt.Errorf("repositories: create cliente: %w", ErrCNPJDuplicado)
		}
		return fmt.Errorf("repositories: create cliente: %w", err)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.Create", "definindo id, err com resultado de chamada a res.LastInsertId")
	id, err := res.LastInsertId()
	vlog.Printf("cliente_repository.go", "ClienteRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create cliente lastInsertId: %w", err)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.Create", "atribuindo c.ClienteIDOrigem = id")
	c.ClienteIDOrigem = id
	return nil
}

// Update atualiza os campos editáveis de um cliente (cliente_id_origem e
// ativo não são alterados por aqui). Retorna ErrNotFound se não existir.
func (r *ClienteRepository) Update(ctx context.Context, db *sql.DB, id int64, c *models.Cliente) error {
	const q = `
		UPDATE clientes
		SET cnpj = ?, razao_social = ?, segmento = ?, cidade = ?, uf = ?, bairro = ?, data_cadastro = ?
		WHERE cliente_id_origem = ?`
	vlog.Printf("cliente_repository.go", "ClienteRepository.Update", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q,
		c.CNPJ,
		c.RazaoSocial,
		c.Segmento,
		c.Cidade,
		c.UF,
		c.Bairro,
		c.DataCadastro,
		id,
	)
	vlog.Printf("cliente_repository.go", "ClienteRepository.Update", "verificando se err != nil")
	if err != nil {
		vlog.Printf("cliente_repository.go", "ClienteRepository.Update", "verificando se isDuplicateKey(err, uqClientesCNPJ)")
		if isDuplicateKey(err, uqClientesCNPJ) {
			return fmt.Errorf("repositories: update cliente: %w", ErrCNPJDuplicado)
		}
		return fmt.Errorf("repositories: update cliente: %w", err)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.Update", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("cliente_repository.go", "ClienteRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update cliente rowsAffected: %w", err)
	}
	vlog.Printf("cliente_repository.go", "ClienteRepository.Update", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCliente(s rowScanner) (*models.Cliente, error) {
	vlog.Printf("cliente_repository.go", "scanCliente", "declarando variável c")
	var c models.Cliente
	vlog.Printf("cliente_repository.go", "scanCliente", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
	if err := s.Scan(
		&c.ClienteIDOrigem,
		&c.CNPJ,
		&c.RazaoSocial,
		&c.Segmento,
		&c.Cidade,
		&c.UF,
		&c.Bairro,
		&c.DataCadastro,
		&c.Ativo,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		vlog.Printf("cliente_repository.go", "scanCliente", "verificando se err == sql.ErrNoRows")
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan cliente: %w", err)
	}
	return &c, nil
}
