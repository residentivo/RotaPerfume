// Command importoportunidades lê dados/crm/oportunidades.csv e importa
// (upsert) os registros na tabela `oportunidades` (ver
// sql/15_ddl_oportunidades.sql).
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importoportunidades
//	cd apis/shared && go run ./cmd/importoportunidades -csv=/caminho/alternativo/oportunidades.csv
//	cd apis/shared && go run ./cmd/importoportunidades -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-oportunidades
//
// Pré-requisito: `clientes` e `vendedores` já devem estar importados/seedados
// (o lookup de cliente_id depende disso; vendedor_id do CSV já corresponde
// 1:1 ao id de vendedores, sem necessidade de lookup — mesmo padrão de
// importcarteiras).
//
// Comportamento:
//  1. Carrega .env da raiz do projeto (mesmo mecanismo dos demais importadores).
//  2. Lê o CSV informado via flag -csv (default: dados/crm/oportunidades.csv,
//     resolvido a partir da raiz do repositório) ou via env
//     OPORTUNIDADES_CSV_PATH.
//  3. Faz parsing linha a linha com TRIM em todos os campos:
//     oportunidade_id,cliente_id,vendedor_id,origem,data_abertura,etapa,
//     probabilidade_pct,valor_estimado,data_fechamento,ciclo_dias,motivo_perda
//     - data_abertura: aceita "2006-01-02" e "02/01/2006".
//     - data_fechamento, ciclo_dias e motivo_perda podem vir vazios (NULL) —
//     oportunidade ainda aberta.
//  4. Resolve `cliente_id` do CSV para o `cliente_id_origem` de `clientes` via
//     lookup pré-carregado em memória (mesmo padrão de importcarteiras).
//  5. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando o próprio
//     `oportunidade_id` (PK AUTO_INCREMENT, valor vindo explicitamente do
//     CSV) como chave de idempotência — mesmo padrão de importpagamentos.
//  6. Loga contadores finais: total de linhas lidas, importadas (insert),
//     atualizadas (update) e com erro — e falha (log.Fatalf) se houver erro
//     de conexão ou de leitura do arquivo.
package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"github.com/rotaperfumes/shared/config"
)

// dateLayouts são os formatos de data aceitos no CSV (mesmo padrão de importcarteiras/importclientes).
var dateLayouts = []string{"2006-01-02", "02/01/2006"}

// oportunidadeRow é uma linha já normalizada do CSV, pronta para o upsert.
type oportunidadeRow struct {
	OportunidadeID   int64
	ClienteIDOrigem  int64
	VendedorID       int64
	Origem           string
	DataAbertura     time.Time
	Etapa            string
	ProbabilidadePct float64
	ValorEstimado    float64
	DataFechamento   *time.Time
	CicloDias        sql.NullInt64
	MotivoPerda      sql.NullString
}

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de oportunidades (default: dados/crm/oportunidades.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("importoportunidades: falha ao carregar config: %v", err)
	}

	csvPath := resolveCSVPath(*csvPathFlag)
	log.Printf("importoportunidades: lendo CSV de %s", csvPath)

	rows, parseErrs, err := readCSV(csvPath)
	if err != nil {
		log.Fatalf("importoportunidades: falha ao ler CSV: %v", err)
	}
	log.Printf("importoportunidades: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	if *dryRun {
		log.Printf("importoportunidades: --dry-run informado, nada foi gravado no banco")
		return
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("importoportunidades: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("importoportunidades: ping no banco falhou: %v", err)
	}

	clienteIDs, err := loadClienteIDsByOrigem(db)
	if err != nil {
		log.Fatalf("importoportunidades: falha ao carregar lookup de clientes: %v", err)
	}
	log.Printf("importoportunidades: %d clientes carregados para lookup", len(clienteIDs))

	inserted, updated, failed := upsertAll(db, rows, clienteIDs)

	log.Printf("importoportunidades: OK — lidos=%d inseridos=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
}

// resolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env OPORTUNIDADES_CSV_PATH > default (dados/crm/oportunidades.csv na raiz do projeto).
func resolveCSVPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("OPORTUNIDADES_CSV_PATH"); v != "" {
		return v
	}
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("importoportunidades: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "dados", "crm", "oportunidades.csv")
}

// readCSV lê e normaliza o arquivo CSV. Linhas malformadas são contadas em
// parseErrs e puladas (não abortam a importação inteira).
func readCSV(path string) (rows []oportunidadeRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 11

	// Descarta o cabeçalho.
	if _, err := r.Read(); err != nil {
		return nil, 0, fmt.Errorf("lendo cabeçalho: %w", err)
	}

	lineNum := 1
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			log.Printf("importoportunidades: linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := parseRow(record)
		if err != nil {
			log.Printf("importoportunidades: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parseRow converte um registro CSV bruto em oportunidadeRow, aplicando TRIM,
// parsing de datas (múltiplos formatos) e tratamento de campos opcionais
// (data_fechamento, ciclo_dias, motivo_perda podem vir vazios).
func parseRow(record []string) (oportunidadeRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	oportunidadeID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return oportunidadeRow{}, fmt.Errorf("oportunidade_id inválido (%q): %w", record[0], err)
	}

	clienteID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return oportunidadeRow{}, fmt.Errorf("cliente_id inválido (%q): %w", record[1], err)
	}

	vendedorID, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return oportunidadeRow{}, fmt.Errorf("vendedor_id inválido (%q): %w", record[2], err)
	}

	origem := record[3]
	if origem == "" {
		return oportunidadeRow{}, fmt.Errorf("origem vazia")
	}

	dataAbertura, err := parseData(record[4])
	if err != nil {
		return oportunidadeRow{}, fmt.Errorf("data_abertura inválida (%q): %w", record[4], err)
	}

	etapa := record[5]
	if etapa == "" {
		return oportunidadeRow{}, fmt.Errorf("etapa vazia")
	}

	probabilidadePct, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return oportunidadeRow{}, fmt.Errorf("probabilidade_pct inválido (%q): %w", record[6], err)
	}

	valorEstimado, err := strconv.ParseFloat(record[7], 64)
	if err != nil {
		return oportunidadeRow{}, fmt.Errorf("valor_estimado inválido (%q): %w", record[7], err)
	}

	var dataFechamento *time.Time
	if record[8] != "" {
		df, err := parseData(record[8])
		if err != nil {
			return oportunidadeRow{}, fmt.Errorf("data_fechamento inválida (%q): %w", record[8], err)
		}
		dataFechamento = &df
	}

	var cicloDias sql.NullInt64
	if record[9] != "" {
		c, err := strconv.ParseInt(record[9], 10, 64)
		if err != nil {
			return oportunidadeRow{}, fmt.Errorf("ciclo_dias inválido (%q): %w", record[9], err)
		}
		cicloDias = sql.NullInt64{Int64: c, Valid: true}
	}

	var motivoPerda sql.NullString
	if record[10] != "" {
		motivoPerda = sql.NullString{String: record[10], Valid: true}
	}

	return oportunidadeRow{
		OportunidadeID:   oportunidadeID,
		ClienteIDOrigem:  clienteID,
		VendedorID:       vendedorID,
		Origem:           origem,
		DataAbertura:     dataAbertura,
		Etapa:            etapa,
		ProbabilidadePct: probabilidadePct,
		ValorEstimado:    valorEstimado,
		DataFechamento:   dataFechamento,
		CicloDias:        cicloDias,
		MotivoPerda:      motivoPerda,
	}, nil
}

// parseData tenta os layouts conhecidos de data observados no CSV real.
func parseData(raw string) (time.Time, error) {
	var lastErr error
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

// loadClienteIDsByOrigem pré-carrega o conjunto de cliente_id_origem
// existentes em `clientes`, usado para validar o cliente_id do CSV. Como
// cliente_id_origem É a PK da tabela (ver sql/09_ddl_clientes.sql), o valor
// usado como FK em oportunidades.cliente_id é o próprio cliente_id_origem —
// o mapa serve apenas para checar existência (identidade origem -> origem).
func loadClienteIDsByOrigem(db *sql.DB) (map[int64]int64, error) {
	rows, err := db.Query("SELECT cliente_id_origem FROM clientes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]int64)
	for rows.Next() {
		var origem int64
		if err := rows.Scan(&origem); err != nil {
			return nil, err
		}
		m[origem] = origem
	}
	return m, rows.Err()
}

// upsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY
// UPDATE, usando o próprio `oportunidade_id` (PK AUTO_INCREMENT, vindo
// explicitamente do CSV) como chave de idempotência — mesmo padrão de
// importpagamentos. Linhas cujo cliente_id não é encontrado no lookup são
// contadas como erro e puladas; vendedor_id não é validado em memória (FK do
// banco garante a integridade e retorna erro no upsert, mesmo padrão de
// importcarteiras).
func upsertAll(db *sql.DB, rows []oportunidadeRow, clienteIDs map[int64]int64) (inserted, updated, failed int) {
	const query = `
		INSERT INTO oportunidades
			(oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cliente_id = VALUES(cliente_id),
			vendedor_id = VALUES(vendedor_id),
			origem = VALUES(origem),
			data_abertura = VALUES(data_abertura),
			etapa = VALUES(etapa),
			probabilidade_pct = VALUES(probabilidade_pct),
			valor_estimado = VALUES(valor_estimado),
			data_fechamento = VALUES(data_fechamento),
			ciclo_dias = VALUES(ciclo_dias),
			motivo_perda = VALUES(motivo_perda)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importoportunidades: prepare falhou: %v", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		clienteID, ok := clienteIDs[row.ClienteIDOrigem]
		if !ok {
			log.Printf("importoportunidades: oportunidade_id=%d: cliente_id=%d não encontrado em clientes.cliente_id_origem",
				row.OportunidadeID, row.ClienteIDOrigem)
			failed++
			continue
		}

		var dataFechamento interface{}
		if row.DataFechamento != nil {
			dataFechamento = row.DataFechamento.Format("2006-01-02")
		}

		var cicloDias interface{}
		if row.CicloDias.Valid {
			cicloDias = row.CicloDias.Int64
		}

		var motivoPerda interface{}
		if row.MotivoPerda.Valid {
			motivoPerda = row.MotivoPerda.String
		}

		result, err := stmt.Exec(
			row.OportunidadeID, clienteID, row.VendedorID, row.Origem,
			row.DataAbertura.Format("2006-01-02"), row.Etapa, row.ProbabilidadePct,
			row.ValorEstimado, dataFechamento, cicloDias, motivoPerda,
		)
		if err != nil {
			log.Printf("importoportunidades: erro no upsert de oportunidade_id=%d (vendedor_id=%d): %v",
				row.OportunidadeID, row.VendedorID, err)
			failed++
			continue
		}
		// MySQL retorna affected rows = 1 para INSERT novo, 2 para UPDATE
		// (quando houve alteração real) via ON DUPLICATE KEY UPDATE.
		affected, _ := result.RowsAffected()
		switch affected {
		case 1:
			inserted++
		case 2:
			updated++
		default:
			// affected == 0: linha já existia e nenhum valor mudou.
			updated++
		}
	}
	return inserted, updated, failed
}

// findProjectRoot sobe a árvore de diretórios a partir do cwd até achar a
// raiz do repositório (identificada por apis/shared/go.mod).
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "apis", "shared", "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("não foi possível localizar apis/shared/go.mod a partir de %s", dir)
}

// loadEnvFromCwd tenta carregar o .env da raiz do projeto subindo diretórios
// a partir do working directory. Não retorna erro — se não achar, segue sem .env.
func loadEnvFromCwd() {
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
				_ = godotenv.Load(filepath.Join(dir, ".env"))
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
}
