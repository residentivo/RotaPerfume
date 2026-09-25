// Command importprodutos lê dados/erp/produtos.csv e importa (upsert) os
// registros na tabela `produtos` (ver sql/10_ddl_produtos.sql).
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importprodutos
//	cd apis/shared && go run ./cmd/importprodutos -csv=/caminho/alternativo/produtos.csv
//	cd apis/shared && go run ./cmd/importprodutos -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-produtos
//
// Comportamento:
//  1. Carrega .env da raiz do projeto (mesmo mecanismo de importclientes).
//  2. Lê o CSV informado via flag -csv (default: dados/erp/produtos.csv, resolvido
//     a partir da raiz do repositório) ou via env PRODUTOS_CSV_PATH.
//  3. O CSV TEM cabeçalho (diferente do CSV de clientes) — a primeira linha é
//     descartada. Colunas: sku,descricao,categoria,marca,nota_olfativa,
//     preco_tabela,custo_unitario,unidade,ativo,data_lancamento.
//  4. Faz parsing linha a linha com TRIM em todos os campos:
//     - preco_tabela/custo_unitario: float.
//     - ativo: 'S' -> true, 'N' -> false (case-insensitive). REGRA: vazio ou
//     valor inválido/ausente vira TRUE (default ATIVO para itens
//     importados) — só fica inativo se o CSV trouxer 'N' explicitamente.
//     - data_lancamento: opcional, formato "2006-01-02"; vazio = NULL.
//  5. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando `sku` como
//     chave de idempotência (UNIQUE KEY na tabela).
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

// dataLancamentoLayout é o formato de data_lancamento aceito no CSV.
const dataLancamentoLayout = "2006-01-02"

// produtoRow é uma linha já normalizada do CSV, pronta para o upsert.
type produtoRow struct {
	SKU            string
	Descricao      string
	Categoria      string
	Marca          string
	NotaOlfativa   string
	PrecoTabela    float64
	CustoUnitario  float64
	Unidade        string
	Ativo          bool
	DataLancamento *time.Time
}

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de produtos (default: dados/erp/produtos.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("importprodutos: falha ao carregar config: %v", err)
	}

	csvPath := resolveCSVPath(*csvPathFlag)
	log.Printf("importprodutos: lendo CSV de %s", csvPath)

	rows, parseErrs, err := readCSV(csvPath)
	if err != nil {
		log.Fatalf("importprodutos: falha ao ler CSV: %v", err)
	}
	log.Printf("importprodutos: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	if *dryRun {
		log.Printf("importprodutos: --dry-run informado, nada foi gravado no banco")
		return
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("importprodutos: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("importprodutos: ping no banco falhou: %v", err)
	}

	inserted, updated, failed := upsertAll(db, rows)

	log.Printf("importprodutos: OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
}

// resolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env PRODUTOS_CSV_PATH > default (dados/erp/produtos.csv na raiz do projeto).
func resolveCSVPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("PRODUTOS_CSV_PATH"); v != "" {
		return v
	}
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("importprodutos: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "dados", "erp", "produtos.csv")
}

// readCSV lê e normaliza o arquivo CSV. Linhas malformadas são contadas em
// parseErrs e puladas (não abortam a importação inteira).
func readCSV(path string) (rows []produtoRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 10

	// Descarta o cabeçalho (este CSV, diferente do de clientes, TEM header).
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
			log.Printf("importprodutos: linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := parseRow(record)
		if err != nil {
			log.Printf("importprodutos: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parseRow converte um registro CSV bruto em produtoRow, aplicando TRIM,
// parsing de floats, conversão de 'S'/'N' para bool (default TRUE quando
// vazio/inválido) e parsing opcional de data_lancamento.
func parseRow(record []string) (produtoRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	sku := record[0]
	descricao := record[1]
	categoria := record[2]
	marca := record[3]
	notaOlfativa := record[4]

	if sku == "" {
		return produtoRow{}, fmt.Errorf("sku vazio")
	}
	if descricao == "" || categoria == "" || marca == "" {
		return produtoRow{}, fmt.Errorf("descricao, categoria ou marca vazios")
	}

	precoTabela, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return produtoRow{}, fmt.Errorf("preco_tabela inválido (%q): %w", record[5], err)
	}

	custoUnitario, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return produtoRow{}, fmt.Errorf("custo_unitario inválido (%q): %w", record[6], err)
	}

	unidade := record[7]
	if unidade == "" {
		return produtoRow{}, fmt.Errorf("unidade vazia")
	}

	ativo := parseAtivo(record[8])

	dataLancamento, err := parseDataLancamento(record[9])
	if err != nil {
		return produtoRow{}, fmt.Errorf("data_lancamento inválida (%q): %w", record[9], err)
	}

	return produtoRow{
		SKU:            sku,
		Descricao:      descricao,
		Categoria:      categoria,
		Marca:          marca,
		NotaOlfativa:   notaOlfativa,
		PrecoTabela:    precoTabela,
		CustoUnitario:  custoUnitario,
		Unidade:        unidade,
		Ativo:          ativo,
		DataLancamento: dataLancamento,
	}, nil
}

// parseAtivo converte 'S'/'N' (case-insensitive) para bool. REGRA DE
// NEGÓCIO: qualquer valor vazio ou não reconhecido é tratado como ATIVO
// (default TRUE) — o padrão exigido para itens importados. Só fica inativo
// quando o CSV traz 'N' explicitamente.
func parseAtivo(raw string) bool {
	switch strings.ToUpper(raw) {
	case "N":
		return false
	default:
		return true
	}
}

// parseDataLancamento faz parsing opcional de data_lancamento. Vazio = NULL (nil).
func parseDataLancamento(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation(dataLancamentoLayout, raw, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// upsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY UPDATE,
// usando sku como chave de idempotência.
func upsertAll(db *sql.DB, rows []produtoRow) (inserted, updated, failed int) {
	const query = `
		INSERT INTO produtos
			(sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento, ativo)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			descricao = VALUES(descricao),
			categoria = VALUES(categoria),
			marca = VALUES(marca),
			nota_olfativa = VALUES(nota_olfativa),
			preco_tabela = VALUES(preco_tabela),
			custo_unitario = VALUES(custo_unitario),
			unidade = VALUES(unidade),
			data_lancamento = VALUES(data_lancamento),
			ativo = VALUES(ativo)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importprodutos: prepare falhou: %v", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		var notaOlfativa any
		if row.NotaOlfativa != "" {
			notaOlfativa = row.NotaOlfativa
		}
		var dataLancamento any
		if row.DataLancamento != nil {
			dataLancamento = row.DataLancamento.Format(dataLancamentoLayout)
		}

		result, err := stmt.Exec(
			row.SKU, row.Descricao, row.Categoria, row.Marca, notaOlfativa,
			row.PrecoTabela, row.CustoUnitario, row.Unidade, dataLancamento, row.Ativo,
		)
		if err != nil {
			log.Printf("importprodutos: erro no upsert de sku=%s: %v", row.SKU, err)
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
