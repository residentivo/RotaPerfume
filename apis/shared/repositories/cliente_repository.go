// Package repositories contém funções puras de acesso a dados (stateless).
// Cliente queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/vlog"
)

// ARQ-04: o contexto Comercial migrou para comercial/comercialmysql. Aqui
// ficam só as métricas de clientes que o Dashboard legado ainda usa
// (CountTotal, CountPorAtivo, CountNovosNoPeriodo, CountPorSegmento,
// CountPorUF), com vendedorID int64 (0 = admin). Saem no ARQ-06, junto com o
// dashboard_repository, para o analyticsmysql com kernel.Escopo.

// ClienteRepository agrupa queries da tabela clientes.
type ClienteRepository struct{}

// NewClienteRepository cria um repositório stateless.
func NewClienteRepository() *ClienteRepository {
	return &ClienteRepository{}
}

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
