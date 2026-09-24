// Command importestoque lê dados/erp/estoque.csv e importa (upsert) os
// registros na tabela `estoque` (ver sql/17_ddl_estoque.sql).
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importestoque
//	cd apis/shared && go run ./cmd/importestoque -csv=/caminho/alternativo/estoque.csv
//	cd apis/shared && go run ./cmd/importestoque -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-estoque
//
// Comportamento:
//  1. Carrega .env da raiz do projeto (mesmo mecanismo de importprodutos).
//  2. Lê o CSV informado via flag -csv (default: dados/erp/estoque.csv, resolvido
//     a partir da raiz do repositório) ou via env ESTOQUE_CSV_PATH.
//  3. O CSV TEM cabeçalho — a primeira linha é descartada. Colunas:
//     data_snapshot,sku,saldo,ruptura.
//  4. Faz parsing linha a linha com TRIM em todos os campos:
//     - data_snapshot: formato "2006-01-02", obrigatório.
//     - sku: obrigatório (deve existir em produtos — FK garante integridade
//     no upsert; erros de FK são contados como erro de upsert, não abortam
//     a importação inteira).
//     - saldo: inteiro.
//     - ruptura: 'S' -> true, 'N' -> false (case-insensitive). REGRA: vazio ou
//     valor inválido/ausente vira FALSE (default sem ruptura) — só fica em
//     ruptura se o CSV trouxer 'S' explicitamente.
//  5. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando a chave
//     composta (data_snapshot, sku) como idempotência (UNIQUE KEY
//     uk_estoque_data_sku na tabela).
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

// dataSnapshotLayout é o formato de data_snapshot aceito no CSV.
const dataSnapshotLayout = "2006-01-02"

// estoqueRow é uma linha já normalizada do CSV, pronta para o upsert.
type estoqueRow struct {
	DataSnapshot time.Time
	SKU          string
	Saldo        int
	Ruptura      bool
}

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de estoque (default: dados/erp/estoque.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("importestoque: falha ao carregar config: %v", err)
	}

	csvPath := resolveCSVPath(*csvPathFlag)
	log.Printf("importestoque: lendo CSV de %s", csvPath)

	rows, parseErrs, err := readCSV(csvPath)
	if err != nil {
		log.Fatalf("importestoque: falha ao ler CSV: %v", err)
	}
	log.Printf("importestoque: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	if *dryRun {
		log.Printf("importestoque: --dry-run informado, nada foi gravado no banco")
		return
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("importestoque: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("importestoque: ping no banco falhou: %v", err)
	}

	inserted, updated, failed := upsertAll(db, rows)

	log.Printf("importestoque: OK — lidos=%d inseridos=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
}

// resolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env ESTOQUE_CSV_PATH > default (dados/erp/estoque.csv na raiz do projeto).
func resolveCSVPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("ESTOQUE_CSV_PATH"); v != "" {
		return v
	}
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("importestoque: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "dados", "erp", "estoque.csv")
}

// readCSV lê e normaliza o arquivo CSV. Linhas malformadas são contadas em
// parseErrs e puladas (não abortam a importação inteira).
func readCSV(path string) (rows []estoqueRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 4

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
			log.Printf("importestoque: linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := parseRow(record)
		if err != nil {
			log.Printf("importestoque: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parseRow converte um registro CSV bruto em estoqueRow, aplicando TRIM,
// parsing de data e inteiro, e conversão de 'S'/'N' para bool (default
// FALSE quando vazio/inválido).
func parseRow(record []string) (estoqueRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	dataSnapshotRaw := record[0]
	sku := record[1]

	if sku == "" {
		return estoqueRow{}, fmt.Errorf("sku vazio")
	}

	dataSnapshot, err := time.ParseInLocation(dataSnapshotLayout, dataSnapshotRaw, time.Local)
	if err != nil {
		return estoqueRow{}, fmt.Errorf("data_snapshot inválida (%q): %w", dataSnapshotRaw, err)
	}

	saldo, err := strconv.Atoi(record[2])
	if err != nil {
		return estoqueRow{}, fmt.Errorf("saldo inválido (%q): %w", record[2], err)
	}

	ruptura := parseRuptura(record[3])

	return estoqueRow{
		DataSnapshot: dataSnapshot,
		SKU:          sku,
		Saldo:        saldo,
		Ruptura:      ruptura,
	}, nil
}

// parseRuptura converte 'S'/'N' (case-insensitive) para bool. Qualquer
// valor vazio ou não reconhecido é tratado como SEM RUPTURA (default
// FALSE) — só fica em ruptura quando o CSV traz 'S' explicitamente.
func parseRuptura(raw string) bool {
	return strings.ToUpper(raw) == "S"
}

// upsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY UPDATE,
// usando a chave composta (data_snapshot, sku) como idempotência. O saldo do
// CSV do ERP é sempre a fonte de verdade para o saldo absoluto do dia,
// mesmo que uma baixa por faturamento tenha gravado esse par
// (data_snapshot, sku) antes.
func upsertAll(db *sql.DB, rows []estoqueRow) (inserted, updated, failed int) {
	const query = `
		INSERT INTO estoque
			(data_snapshot, sku, saldo, ruptura)
		VALUES
			(?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			saldo = VALUES(saldo),
			ruptura = VALUES(ruptura)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importestoque: prepare falhou: %v", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		result, err := stmt.Exec(
			row.DataSnapshot.Format(dataSnapshotLayout), row.SKU, row.Saldo, row.Ruptura,
		)
		if err != nil {
			log.Printf("importestoque: erro no upsert de data_snapshot=%s sku=%s: %v",
				row.DataSnapshot.Format(dataSnapshotLayout), row.SKU, err)
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
