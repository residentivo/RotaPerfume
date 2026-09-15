// Command importcarteiras lê dados/crm/carteira.csv e importa (upsert) os
// registros na tabela `carteiras` (ver sql/14_ddl_carteiras.sql).
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importcarteiras
//	cd apis/shared && go run ./cmd/importcarteiras -csv=/caminho/alternativo/carteira.csv
//	cd apis/shared && go run ./cmd/importcarteiras -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-carteiras
//
// Pré-requisito: `clientes` e `vendedores` já devem estar importados/seedados
// (o lookup de cliente_id depende disso; vendedor_id do CSV já corresponde
// 1:1 ao id de vendedores, sem necessidade de lookup).
//
// Comportamento:
//  1. Carrega .env da raiz do projeto (mesmo mecanismo de seedusers/resetpassword).
//  2. Lê o CSV informado via flag -csv (default: dados/crm/carteira.csv, resolvido
//     a partir da raiz do repositório) ou via env CARTEIRAS_CSV_PATH.
//  3. Faz parsing linha a linha com TRIM em todos os campos:
//     carteira_id,cliente_id,vendedor_id,data_inicio,data_fim
//     - data_inicio: aceita "2006-01-02" e "02/01/2006".
//     - data_fim: mesmos formatos, mas pode vir vazio (vínculo ainda ativo).
//  4. Resolve `cliente_id` do CSV para o `id` interno de `clientes` via lookup
//     em `clientes.cliente_id_origem` (pré-carregado em memória).
//  5. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando
//     `carteira_id_origem` como chave de idempotência (UNIQUE KEY na tabela).
//  6. Loga contadores finais: total de linhas lidas, importadas (insert),
//     atualizadas (update), ignoradas e com erro — e falha (log.Fatalf) se
//     houver erro de conexão ou de leitura do arquivo.
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

// dateLayouts são os formatos de data aceitos no CSV (mesmo padrão de importclientes).
var dateLayouts = []string{"2006-01-02", "02/01/2006"}

// carteiraRow é uma linha já normalizada do CSV, pronta para o upsert.
type carteiraRow struct {
	CarteiraIDOrigem int64
	ClienteIDOrigem  int64
	VendedorID       int64
	DataInicio       time.Time
	DataFim          *time.Time
}

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de carteiras (default: dados/crm/carteira.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("importcarteiras: falha ao carregar config: %v", err)
	}

	csvPath := resolveCSVPath(*csvPathFlag)
	log.Printf("importcarteiras: lendo CSV de %s", csvPath)

	rows, parseErrs, err := readCSV(csvPath)
	if err != nil {
		log.Fatalf("importcarteiras: falha ao ler CSV: %v", err)
	}
	log.Printf("importcarteiras: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	if *dryRun {
		log.Printf("importcarteiras: --dry-run informado, nada foi gravado no banco")
		return
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("importcarteiras: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("importcarteiras: ping no banco falhou: %v", err)
	}

	clienteIDs, err := loadClienteIDsByOrigem(db)
	if err != nil {
		log.Fatalf("importcarteiras: falha ao carregar lookup de clientes: %v", err)
	}
	log.Printf("importcarteiras: %d clientes carregados para lookup", len(clienteIDs))

	inserted, updated, failed := upsertAll(db, rows, clienteIDs)

	log.Printf("importcarteiras: OK — lidos=%d inseridos=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
}

// resolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env CARTEIRAS_CSV_PATH > default (dados/crm/carteira.csv na raiz do projeto).
func resolveCSVPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("CARTEIRAS_CSV_PATH"); v != "" {
		return v
	}
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("importcarteiras: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "dados", "crm", "carteira.csv")
}

// readCSV lê e normaliza o arquivo CSV. Linhas malformadas são contadas em
// parseErrs e puladas (não abortam a importação inteira).
func readCSV(path string) (rows []carteiraRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 5

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
			log.Printf("importcarteiras: linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := parseRow(record)
		if err != nil {
			log.Printf("importcarteiras: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parseRow converte um registro CSV bruto em carteiraRow, aplicando TRIM e
// parsing de datas (múltiplos formatos, data_fim opcional).
func parseRow(record []string) (carteiraRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	carteiraID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return carteiraRow{}, fmt.Errorf("carteira_id inválido (%q): %w", record[0], err)
	}

	clienteID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return carteiraRow{}, fmt.Errorf("cliente_id inválido (%q): %w", record[1], err)
	}

	vendedorID, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return carteiraRow{}, fmt.Errorf("vendedor_id inválido (%q): %w", record[2], err)
	}

	dataInicio, err := parseData(record[3])
	if err != nil {
		return carteiraRow{}, fmt.Errorf("data_inicio inválida (%q): %w", record[3], err)
	}

	var dataFim *time.Time
	if record[4] != "" {
		df, err := parseData(record[4])
		if err != nil {
			return carteiraRow{}, fmt.Errorf("data_fim inválida (%q): %w", record[4], err)
		}
		dataFim = &df
	}

	return carteiraRow{
		CarteiraIDOrigem: carteiraID,
		ClienteIDOrigem:  clienteID,
		VendedorID:       vendedorID,
		DataInicio:       dataInicio,
		DataFim:          dataFim,
	}, nil
}

// parseData tenta os layouts conhecidos de data observados no CSV real.
func parseData(raw string) (time.Time, error) {
	var lastErr error
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}

// loadClienteIDsByOrigem pré-carrega o mapa cliente_id_origem -> id, usado
// para resolver o cliente_id do CSV (que referencia a origem, não o id interno).
func loadClienteIDsByOrigem(db *sql.DB) (map[int64]int64, error) {
	rows, err := db.Query("SELECT id, cliente_id_origem FROM clientes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64]int64)
	for rows.Next() {
		var id, origem int64
		if err := rows.Scan(&id, &origem); err != nil {
			return nil, err
		}
		m[origem] = id
	}
	return m, rows.Err()
}

// upsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY UPDATE,
// usando carteira_id_origem como chave de idempotência. Linhas cujo cliente_id
// não é encontrado no lookup são contadas como erro e puladas.
func upsertAll(db *sql.DB, rows []carteiraRow, clienteIDs map[int64]int64) (inserted, updated, failed int) {
	const query = `
		INSERT INTO carteiras
			(carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim)
		VALUES
			(?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cliente_id = VALUES(cliente_id),
			vendedor_id = VALUES(vendedor_id),
			data_inicio = VALUES(data_inicio),
			data_fim = VALUES(data_fim)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importcarteiras: prepare falhou: %v", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		clienteID, ok := clienteIDs[row.ClienteIDOrigem]
		if !ok {
			log.Printf("importcarteiras: carteira_id_origem=%d: cliente_id=%d não encontrado em clientes.cliente_id_origem",
				row.CarteiraIDOrigem, row.ClienteIDOrigem)
			failed++
			continue
		}

		var dataFim interface{}
		if row.DataFim != nil {
			dataFim = row.DataFim.Format("2006-01-02")
		}

		result, err := stmt.Exec(
			row.CarteiraIDOrigem, clienteID, row.VendedorID,
			row.DataInicio.Format("2006-01-02"), dataFim,
		)
		if err != nil {
			log.Printf("importcarteiras: erro no upsert de carteira_id_origem=%d: %v", row.CarteiraIDOrigem, err)
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
