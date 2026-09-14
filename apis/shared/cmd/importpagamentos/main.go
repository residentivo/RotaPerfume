// Command importpagamentos lê dados/erp/pagamentos.csv e importa (upsert)
// os registros na tabela `pagamentos` (ver sql/12_ddl_pagamentos.sql).
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importpagamentos
//	cd apis/shared && go run ./cmd/importpagamentos -pagamentos-csv=/caminho/pagamentos.csv
//	cd apis/shared && go run ./cmd/importpagamentos -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-pagamentos
//
// Comportamento:
//  1. Carrega .env da raiz do projeto (mesmo mecanismo de importclientes/
//     importprodutos/importpedidos).
//  2. Lê pagamentos.csv (default dados/erp/pagamentos.csv, COM header):
//     pagamento_id,pedido_id,forma_pagamento,parcelas,valor,taxa_pct,
//     valor_liquido,data_vencimento,data_pagamento,status_pagamento
//     - `pedido_id` do CSV é resolvido via lookup em `pedidos.pedido_id_origem`
//     (pré-carregado em memória), pois `pedidos` mantém a PK interna `id`
//     desacoplada da origem (padrão diferente de `pagamentos`, cuja PK É o
//     próprio `pagamento_id` do CSV — ver sql/12_ddl_pagamentos.sql).
//     - `data_pagamento` vazia no CSV é gravada como NULL (pagamento ainda
//     não ocorreu — status "Em aberto" ou "Inadimplente").
//     - Linha com pedido não encontrado é contada como erro e pulada (não
//     aborta a importação).
//     - Upsert via INSERT ... ON DUPLICATE KEY UPDATE usando o próprio
//     `pagamento_id` (PK, já vindo do CSV) como chave de idempotência — o
//     valor é inserido explicitamente, NUNCA gerado pelo AUTO_INCREMENT.
//  3. Loga contadores finais: linhas lidas, inseridas, atualizadas, erros de
//     parsing e erros de FK/upsert.
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

// dataLayout é o formato canônico das colunas de data em pagamentos.csv
// (data_vencimento, data_pagamento) e usado para gravação no banco.
const dataLayout = "2006-01-02"

// pagamentoRow é uma linha já normalizada de pagamentos.csv, pronta para o upsert.
type pagamentoRow struct {
	PagamentoID     int64
	PedidoIDOrigem  int64
	FormaPagamento  string
	Parcelas        int
	Valor           float64
	TaxaPct         float64
	ValorLiquido    float64
	DataVencimento  string
	DataPagamento   sql.NullString
	StatusPagamento string
}

func main() {
	pagamentosCSVFlag := flag.String("pagamentos-csv", "", "caminho do CSV de pagamentos (default: dados/erp/pagamentos.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("importpagamentos: falha ao carregar config: %v", err)
	}

	pagamentosCSVPath := resolveCSVPath(*pagamentosCSVFlag, "PAGAMENTOS_CSV_PATH", "pagamentos.csv")

	log.Printf("importpagamentos: lendo CSV de pagamentos: %s", pagamentosCSVPath)
	rows, parseErrs, err := readPagamentosCSV(pagamentosCSVPath)
	if err != nil {
		log.Fatalf("importpagamentos: falha ao ler CSV de pagamentos: %v", err)
	}
	log.Printf("importpagamentos: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	if *dryRun {
		log.Printf("importpagamentos: --dry-run informado, nada foi gravado no banco")
		return
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("importpagamentos: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("importpagamentos: ping no banco falhou: %v", err)
	}

	pedidoIDs, err := loadPedidoIDsByOrigem(db)
	if err != nil {
		log.Fatalf("importpagamentos: falha ao carregar pedidos: %v", err)
	}
	log.Printf("importpagamentos: %d pedidos carregados para lookup", len(pedidoIDs))

	inserted, updated, failed := upsertPagamentos(db, rows, pedidoIDs)
	log.Printf("importpagamentos: concluído — lidos=%d inseridos=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
}

// resolveCSVPath decide o caminho final do CSV, na ordem:
// flag > env > default (dados/erp/<fileName> na raiz do projeto).
func resolveCSVPath(flagValue, envVar, fileName string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("importpagamentos: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "dados", "erp", fileName)
}

// ---------------------------------------------------------------
// Leitura e parsing: pagamentos.csv
// ---------------------------------------------------------------

func readPagamentosCSV(path string) (rows []pagamentoRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 10

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
			log.Printf("importpagamentos: pagamentos.csv linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := parsePagamentoRow(record)
		if err != nil {
			log.Printf("importpagamentos: pagamentos.csv linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parsePagamentoRow converte um registro CSV bruto (pagamento_id,pedido_id,
// forma_pagamento,parcelas,valor,taxa_pct,valor_liquido,data_vencimento,
// data_pagamento,status_pagamento) em pagamentoRow.
func parsePagamentoRow(record []string) (pagamentoRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	pagamentoID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return pagamentoRow{}, fmt.Errorf("pagamento_id inválido (%q): %w", record[0], err)
	}

	pedidoID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return pagamentoRow{}, fmt.Errorf("pedido_id inválido (%q): %w", record[1], err)
	}

	formaPagamento := record[2]
	if !isValidFormaPagamento(formaPagamento) {
		return pagamentoRow{}, fmt.Errorf("forma_pagamento inválida (%q)", formaPagamento)
	}

	parcelas, err := strconv.Atoi(record[3])
	if err != nil {
		return pagamentoRow{}, fmt.Errorf("parcelas inválido (%q): %w", record[3], err)
	}

	valor, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return pagamentoRow{}, fmt.Errorf("valor inválido (%q): %w", record[4], err)
	}

	taxaPct, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return pagamentoRow{}, fmt.Errorf("taxa_pct inválido (%q): %w", record[5], err)
	}

	valorLiquido, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return pagamentoRow{}, fmt.Errorf("valor_liquido inválido (%q): %w", record[6], err)
	}

	dataVencimento, err := parseData(record[7])
	if err != nil {
		return pagamentoRow{}, fmt.Errorf("data_vencimento inválida (%q): %w", record[7], err)
	}

	var dataPagamento sql.NullString
	if record[8] != "" {
		d, err := parseData(record[8])
		if err != nil {
			return pagamentoRow{}, fmt.Errorf("data_pagamento inválida (%q): %w", record[8], err)
		}
		dataPagamento = sql.NullString{String: d, Valid: true}
	}

	statusPagamento := record[9]
	if !isValidStatusPagamento(statusPagamento) {
		return pagamentoRow{}, fmt.Errorf("status_pagamento inválido (%q)", statusPagamento)
	}

	return pagamentoRow{
		PagamentoID:     pagamentoID,
		PedidoIDOrigem:  pedidoID,
		FormaPagamento:  formaPagamento,
		Parcelas:        parcelas,
		Valor:           valor,
		TaxaPct:         taxaPct,
		ValorLiquido:    valorLiquido,
		DataVencimento:  dataVencimento,
		DataPagamento:   dataPagamento,
		StatusPagamento: statusPagamento,
	}, nil
}

// parseData valida e normaliza uma data no layout ISO (2006-01-02).
func parseData(v string) (string, error) {
	t, err := time.Parse(dataLayout, v)
	if err != nil {
		return "", err
	}
	return t.Format(dataLayout), nil
}

func isValidFormaPagamento(v string) bool {
	switch v {
	case "Boleto 14 dias", "Boleto 28 dias", "Cartão de crédito", "Cartão de débito", "Cheque a prazo", "Dinheiro", "PIX":
		return true
	default:
		return false
	}
}

func isValidStatusPagamento(v string) bool {
	switch v {
	case "Em aberto", "Inadimplente", "Pago", "Pago com atraso":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------
// Lookup (pré-carregado em memória, mesmo padrão de importpedidos).
// ---------------------------------------------------------------

func loadPedidoIDsByOrigem(db *sql.DB) (map[int64]int64, error) {
	rows, err := db.Query("SELECT id, pedido_id_origem FROM pedidos")
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

// ---------------------------------------------------------------
// Upsert
// ---------------------------------------------------------------

// upsertPagamentos grava as linhas de pagamentos no banco via INSERT ... ON
// DUPLICATE KEY UPDATE, usando o próprio `pagamento_id` (PK, vindo do CSV)
// como chave de idempotência — diferente de pedidos/itens_pedido, aqui NÃO
// há coluna `id` desacoplada nem `pagamento_id_origem` (ver
// sql/12_ddl_pagamentos.sql). Linhas cujo pedido_id não é encontrado no
// lookup são contadas como erro e puladas (não abortam a importação).
func upsertPagamentos(db *sql.DB, rows []pagamentoRow, pedidoIDs map[int64]int64) (inserted, updated, failed int) {
	const query = `
		INSERT INTO pagamentos
			(pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			pedido_id = VALUES(pedido_id),
			forma_pagamento = VALUES(forma_pagamento),
			parcelas = VALUES(parcelas),
			valor = VALUES(valor),
			taxa_pct = VALUES(taxa_pct),
			valor_liquido = VALUES(valor_liquido),
			data_vencimento = VALUES(data_vencimento),
			data_pagamento = VALUES(data_pagamento),
			status_pagamento = VALUES(status_pagamento)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importpagamentos: prepare (pagamentos) falhou: %v", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		pedidoID, ok := pedidoIDs[row.PedidoIDOrigem]
		if !ok {
			log.Printf("importpagamentos: pagamento_id=%d: pedido_id=%d não encontrado em pedidos.pedido_id_origem",
				row.PagamentoID, row.PedidoIDOrigem)
			failed++
			continue
		}

		var dataPagamento interface{}
		if row.DataPagamento.Valid {
			dataPagamento = row.DataPagamento.String
		}

		result, err := stmt.Exec(
			row.PagamentoID, pedidoID, row.FormaPagamento, row.Parcelas,
			row.Valor, row.TaxaPct, row.ValorLiquido,
			row.DataVencimento, dataPagamento, row.StatusPagamento,
		)
		if err != nil {
			log.Printf("importpagamentos: erro no upsert de pagamento_id=%d: %v", row.PagamentoID, err)
			failed++
			continue
		}
		affected, _ := result.RowsAffected()
		switch affected {
		case 1:
			inserted++
		case 2:
			updated++
		default:
			updated++
		}
	}
	return inserted, updated, failed
}

// ---------------------------------------------------------------
// Helpers de ambiente (mesmo padrão de importclientes/importprodutos/importpedidos)
// ---------------------------------------------------------------

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
