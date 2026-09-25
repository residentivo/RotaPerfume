// Command importvisitas lê dados/crm/visitas.csv e importa (upsert) os
// registros na tabela `visitas` (ver sql/16_ddl_visitas.sql).
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importvisitas
//	cd apis/shared && go run ./cmd/importvisitas -csv=/caminho/alternativo/visitas.csv
//	cd apis/shared && go run ./cmd/importvisitas -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-visitas
//
// Pré-requisito: `clientes` e `vendedores` já devem estar importados/seedados
// (o lookup de cliente_id depende disso; vendedor_id do CSV já corresponde
// 1:1 ao id de vendedores, sem necessidade de lookup — mesmo padrão de
// importoportunidades).
//
// Comportamento:
//  1. Carrega .env da raiz do projeto (mesmo mecanismo dos demais importadores).
//  2. Lê o CSV informado via flag -csv (default: dados/crm/visitas.csv,
//     resolvido a partir da raiz do repositório) ou via env
//     VISITAS_CSV_PATH.
//  3. Faz parsing linha a linha com TRIM em todos os campos:
//     visita_id,cliente_id,vendedor_id,data_visita,resultado,duracao_min
//     - data_visita: aceita "2006-01-02" e "02/01/2006".
//  4. Resolve `cliente_id` do CSV para o `cliente_id_origem` de `clientes` via
//     lookup pré-carregado em memória (mesmo padrão de importoportunidades).
//  5. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando o próprio
//     `visita_id` (PK AUTO_INCREMENT, valor vindo explicitamente do CSV)
//     como chave de idempotência — mesmo padrão de importoportunidades.
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

	"github.com/rotaperfumes/shared/cmd/internal/clientesdedup"
	"github.com/rotaperfumes/shared/config"
)

// dateLayouts são os formatos de data aceitos no CSV (mesmo padrão de importoportunidades).
var dateLayouts = []string{"2006-01-02", "02/01/2006"}

// visitaRow é uma linha já normalizada do CSV, pronta para o upsert.
type visitaRow struct {
	VisitaID        int64
	ClienteIDOrigem int64
	VendedorID      int64
	DataVisita      time.Time
	Resultado       string
	DuracaoMin      int64
}

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de visitas (default: dados/crm/visitas.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("importvisitas: falha ao carregar config: %v", err)
	}

	csvPath := resolveCSVPath(*csvPathFlag)
	log.Printf("importvisitas: lendo CSV de %s", csvPath)

	rows, parseErrs, err := readCSV(csvPath)
	if err != nil {
		log.Fatalf("importvisitas: falha ao ler CSV: %v", err)
	}
	log.Printf("importvisitas: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	if *dryRun {
		log.Printf("importvisitas: --dry-run informado, nada foi gravado no banco")
		return
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("importvisitas: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("importvisitas: ping no banco falhou: %v", err)
	}

	clienteIDs, err := loadClienteIDsByOrigem(db)
	if err != nil {
		log.Fatalf("importvisitas: falha ao carregar lookup de clientes: %v", err)
	}
	log.Printf("importvisitas: %d clientes carregados para lookup", len(clienteIDs))
	// NEG-01: cópias de CNPJ duplicado no clientes.csv apontam para o sobrevivente.
	projectRoot, _ := findProjectRoot()
	clientesdedup.Redirecionar("importvisitas", projectRoot, clienteIDs)

	inserted, updated, failed := upsertAll(db, rows, clienteIDs)

	log.Printf("importvisitas: OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
}

// resolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env VISITAS_CSV_PATH > default (dados/crm/visitas.csv na raiz do projeto).
func resolveCSVPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("VISITAS_CSV_PATH"); v != "" {
		return v
	}
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("importvisitas: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "dados", "crm", "visitas.csv")
}

// readCSV lê e normaliza o arquivo CSV. Linhas malformadas são contadas em
// parseErrs e puladas (não abortam a importação inteira).
func readCSV(path string) (rows []visitaRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 6

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
			log.Printf("importvisitas: linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := parseRow(record)
		if err != nil {
			log.Printf("importvisitas: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parseRow converte um registro CSV bruto em visitaRow, aplicando TRIM e
// parsing de data (múltiplos formatos).
func parseRow(record []string) (visitaRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	visitaID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return visitaRow{}, fmt.Errorf("visita_id inválido (%q): %w", record[0], err)
	}

	clienteID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return visitaRow{}, fmt.Errorf("cliente_id inválido (%q): %w", record[1], err)
	}

	vendedorID, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return visitaRow{}, fmt.Errorf("vendedor_id inválido (%q): %w", record[2], err)
	}

	dataVisita, err := parseData(record[3])
	if err != nil {
		return visitaRow{}, fmt.Errorf("data_visita inválida (%q): %w", record[3], err)
	}

	resultado := record[4]
	if resultado == "" {
		return visitaRow{}, fmt.Errorf("resultado vazio")
	}

	duracaoMin, err := strconv.ParseInt(record[5], 10, 64)
	if err != nil {
		return visitaRow{}, fmt.Errorf("duracao_min inválido (%q): %w", record[5], err)
	}

	return visitaRow{
		VisitaID:        visitaID,
		ClienteIDOrigem: clienteID,
		VendedorID:      vendedorID,
		DataVisita:      dataVisita,
		Resultado:       resultado,
		DuracaoMin:      duracaoMin,
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
// usado como FK em visitas.cliente_id é o próprio cliente_id_origem — o mapa
// serve apenas para checar existência (identidade origem -> origem).
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
// UPDATE, usando o próprio `visita_id` (PK AUTO_INCREMENT, vindo
// explicitamente do CSV) como chave de idempotência — mesmo padrão de
// importoportunidades. Linhas cujo cliente_id não é encontrado no lookup são
// contadas como erro e puladas; vendedor_id não é validado em memória (FK do
// banco garante a integridade e retorna erro no upsert, mesmo padrão de
// importoportunidades).
func upsertAll(db *sql.DB, rows []visitaRow, clienteIDs map[int64]int64) (inserted, updated, failed int) {
	const query = `
		INSERT INTO visitas
			(visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min)
		VALUES
			(?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cliente_id = VALUES(cliente_id),
			vendedor_id = VALUES(vendedor_id),
			data_visita = VALUES(data_visita),
			resultado = VALUES(resultado),
			duracao_min = VALUES(duracao_min)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importvisitas: prepare falhou: %v", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		clienteID, ok := clienteIDs[row.ClienteIDOrigem]
		if !ok {
			log.Printf("importvisitas: visita_id=%d: cliente_id=%d não encontrado em clientes.cliente_id_origem",
				row.VisitaID, row.ClienteIDOrigem)
			failed++
			continue
		}

		result, err := stmt.Exec(
			row.VisitaID, clienteID, row.VendedorID,
			row.DataVisita.Format("2006-01-02"), row.Resultado, row.DuracaoMin,
		)
		if err != nil {
			log.Printf("importvisitas: erro no upsert de visita_id=%d (vendedor_id=%d): %v",
				row.VisitaID, row.VendedorID, err)
			failed++
			continue
		}
		// Com clientFoundRows=true no DSN (BUG-04), o MySQL devolve, via ON
		// DUPLICATE KEY UPDATE: 1 = INSERT novo OU linha já existente sem
		// alteração; 2 = UPDATE com alteração real. Por isso o contador
		// "inseridos" é rotulado como inseridos_ou_inalterados.
		affected, _ := result.RowsAffected()
		switch affected {
		case 1:
			inserted++
		case 2:
			updated++
		default:
			// affected == 0: não ocorre com clientFoundRows=true (mantido por robustez).
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
