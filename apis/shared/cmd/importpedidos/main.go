// Command importpedidos lê dados/erp/pedidos.csv e dados/erp/itens_pedido.csv
// e importa (upsert) os registros nas tabelas `pedidos` e `itens_pedido`
// (ver sql/04_ddl_pedidos.sql e sql/11_ddl_itens_pedido.sql), nessa ordem
// (itens dependem dos pedidos já importados).
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importpedidos
//	cd apis/shared && go run ./cmd/importpedidos -pedidos-csv=/caminho/pedidos.csv -itens-csv=/caminho/itens_pedido.csv
//	cd apis/shared && go run ./cmd/importpedidos -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-pedidos
//
// Comportamento:
//  1. Carrega .env da raiz do projeto (mesmo mecanismo de importclientes/importprodutos).
//  2. FASE 1 — pedidos.csv (default dados/erp/pedidos.csv, COM header):
//     pedido_id,cliente_id,vendedor_id,data_pedido,canal,status,valor_total
//     - `cliente_id` do CSV é resolvido via lookup em `clientes.cliente_id_origem`
//     (pré-carregado em memória). Linha sem cliente correspondente é
//     contada como erro e pulada (não aborta a importação).
//     - `vendedor_id` do CSV é usado DIRETO como FK para `vendedores.id`
//     (a seed de vendedores usa os mesmos IDs 1..42 do CSV de pedidos).
//     - Upsert via INSERT ... ON DUPLICATE KEY UPDATE usando
//     `pedido_id_origem` como chave de idempotência.
//  3. FASE 2 — itens_pedido.csv (default dados/erp/itens_pedido.csv, COM header):
//     item_id,pedido_id,sku,quantidade,preco_praticado,desconto_pct,valor_bruto
//     - `pedido_id` do CSV é resolvido via lookup em `pedidos.pedido_id_origem`
//     (pré-carregado em memória, incluindo pedidos já existentes antes desta
//     execução — não apenas os importados na Fase 1).
//     - `sku` do CSV é resolvido via lookup em `produtos.sku` (pré-carregado
//     em memória).
//     - Linha com pedido ou produto não encontrado é contada como erro e
//     pulada (não aborta a importação).
//     - Upsert via INSERT ... ON DUPLICATE KEY UPDATE usando
//     `item_id_origem` como chave de idempotência.
//  4. Loga contadores finais de cada fase: linhas lidas, inseridas,
//     atualizadas, erros de parsing e erros de FK/upsert.
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

// dataPedidoLayout é o formato canônico usado para gravar data_pedido no banco.
const dataPedidoLayout = "2006-01-02"

// dataPedidoLayoutBR é um formato alternativo (DD/MM/YYYY) observado em parte
// das linhas de pedidos.csv. O parser tenta ISO primeiro e, se falhar, tenta
// este formato antes de contar a linha como erro de parsing.
const dataPedidoLayoutBR = "02/01/2006"

// pedidoRow é uma linha já normalizada de pedidos.csv, pronta para o upsert.
type pedidoRow struct {
	PedidoIDOrigem  int64
	ClienteIDOrigem int64
	VendedorID      int64
	DataPedido      time.Time
	Canal           string
	Status          string
	ValorTotal      float64
}

// itemPedidoRow é uma linha já normalizada de itens_pedido.csv, pronta para o upsert.
type itemPedidoRow struct {
	ItemIDOrigem   int64
	PedidoIDOrigem int64
	SKU            string
	Quantidade     int
	PrecoPraticado float64
	DescontoPct    float64
	ValorBruto     float64
}

func main() {
	pedidosCSVFlag := flag.String("pedidos-csv", "", "caminho do CSV de pedidos (default: dados/erp/pedidos.csv na raiz do projeto)")
	itensCSVFlag := flag.String("itens-csv", "", "caminho do CSV de itens de pedido (default: dados/erp/itens_pedido.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("importpedidos: falha ao carregar config: %v", err)
	}

	pedidosCSVPath := resolveCSVPath(*pedidosCSVFlag, "PEDIDOS_CSV_PATH", "pedidos.csv")
	itensCSVPath := resolveCSVPath(*itensCSVFlag, "ITENS_PEDIDO_CSV_PATH", "itens_pedido.csv")

	// -----------------------------------------------------------
	// FASE 1: pedidos.csv
	// -----------------------------------------------------------
	log.Printf("importpedidos: [fase 1/2] lendo CSV de pedidos: %s", pedidosCSVPath)
	pedidoRows, pedidoParseErrs, err := readPedidosCSV(pedidosCSVPath)
	if err != nil {
		log.Fatalf("importpedidos: falha ao ler CSV de pedidos: %v", err)
	}
	log.Printf("importpedidos: [fase 1/2] %d linhas válidas lidas, %d linhas com erro de parsing", len(pedidoRows), pedidoParseErrs)

	if *dryRun {
		log.Printf("importpedidos: [fase 2/2] lendo CSV de itens de pedido: %s", itensCSVPath)
		itemRows, itemParseErrs, err := readItensPedidoCSV(itensCSVPath)
		if err != nil {
			log.Fatalf("importpedidos: falha ao ler CSV de itens de pedido: %v", err)
		}
		log.Printf("importpedidos: [fase 2/2] %d linhas válidas lidas, %d linhas com erro de parsing", len(itemRows), itemParseErrs)
		log.Printf("importpedidos: --dry-run informado, nada foi gravado no banco")
		return
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("importpedidos: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("importpedidos: ping no banco falhou: %v", err)
	}

	clienteIDs, err := loadClienteIDsByOrigem(db)
	if err != nil {
		log.Fatalf("importpedidos: falha ao carregar clientes: %v", err)
	}
	log.Printf("importpedidos: %d clientes carregados para lookup", len(clienteIDs))

	pInserted, pUpdated, pFailed := upsertPedidos(db, pedidoRows, clienteIDs)
	log.Printf("importpedidos: [fase 1/2] OK — lidos=%d inseridos=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(pedidoRows), pInserted, pUpdated, pFailed, pedidoParseErrs)

	// -----------------------------------------------------------
	// FASE 2: itens_pedido.csv
	// -----------------------------------------------------------
	log.Printf("importpedidos: [fase 2/2] lendo CSV de itens de pedido: %s", itensCSVPath)
	itemRows, itemParseErrs, err := readItensPedidoCSV(itensCSVPath)
	if err != nil {
		log.Fatalf("importpedidos: falha ao ler CSV de itens de pedido: %v", err)
	}
	log.Printf("importpedidos: [fase 2/2] %d linhas válidas lidas, %d linhas com erro de parsing", len(itemRows), itemParseErrs)

	pedidoIDs, err := loadPedidoIDsByOrigem(db)
	if err != nil {
		log.Fatalf("importpedidos: falha ao carregar pedidos: %v", err)
	}
	log.Printf("importpedidos: %d pedidos carregados para lookup", len(pedidoIDs))

	produtoIDs, err := loadProdutoIDsBySKU(db)
	if err != nil {
		log.Fatalf("importpedidos: falha ao carregar produtos: %v", err)
	}
	log.Printf("importpedidos: %d produtos carregados para lookup", len(produtoIDs))

	iInserted, iUpdated, iFailed := upsertItensPedido(db, itemRows, pedidoIDs, produtoIDs)
	log.Printf("importpedidos: [fase 2/2] OK — lidos=%d inseridos=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(itemRows), iInserted, iUpdated, iFailed, itemParseErrs)

	log.Printf("importpedidos: concluído — pedidos(inseridos=%d atualizados=%d erros=%d) itens(inseridos=%d atualizados=%d erros=%d)",
		pInserted, pUpdated, pFailed, iInserted, iUpdated, iFailed)
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
		log.Fatalf("importpedidos: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "dados", "erp", fileName)
}

// ---------------------------------------------------------------
// Leitura e parsing: pedidos.csv
// ---------------------------------------------------------------

func readPedidosCSV(path string) (rows []pedidoRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 7

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
			log.Printf("importpedidos: pedidos.csv linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := parsePedidoRow(record)
		if err != nil {
			log.Printf("importpedidos: pedidos.csv linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parsePedidoRow converte um registro CSV bruto (pedido_id,cliente_id,
// vendedor_id,data_pedido,canal,status,valor_total) em pedidoRow.
func parsePedidoRow(record []string) (pedidoRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	pedidoID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return pedidoRow{}, fmt.Errorf("pedido_id inválido (%q): %w", record[0], err)
	}

	clienteID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return pedidoRow{}, fmt.Errorf("cliente_id inválido (%q): %w", record[1], err)
	}

	vendedorID, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return pedidoRow{}, fmt.Errorf("vendedor_id inválido (%q): %w", record[2], err)
	}

	dataPedido, err := parseDataPedido(record[3])
	if err != nil {
		return pedidoRow{}, fmt.Errorf("data_pedido inválida (%q): %w", record[3], err)
	}

	canal := record[4]
	if !isValidCanal(canal) {
		return pedidoRow{}, fmt.Errorf("canal inválido (%q)", canal)
	}

	status := record[5]
	if !isValidStatus(status) {
		return pedidoRow{}, fmt.Errorf("status inválido (%q)", status)
	}

	valorTotal, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return pedidoRow{}, fmt.Errorf("valor_total inválido (%q): %w", record[6], err)
	}

	return pedidoRow{
		PedidoIDOrigem:  pedidoID,
		ClienteIDOrigem: clienteID,
		VendedorID:      vendedorID,
		DataPedido:      dataPedido,
		Canal:           canal,
		Status:          status,
		ValorTotal:      valorTotal,
	}, nil
}

// parseDataPedido tenta o layout ISO (2006-01-02) primeiro e, se falhar,
// tenta o layout BR (02/01/2006), pois pedidos.csv mistura ambos os formatos.
// Retorna erro se nenhum dos dois formatos casar.
func parseDataPedido(v string) (time.Time, error) {
	if t, err := time.Parse(dataPedidoLayout, v); err == nil {
		return t, nil
	}
	return time.Parse(dataPedidoLayoutBR, v)
}

func isValidCanal(v string) bool {
	switch v {
	case "App", "Telefone", "Visita", "WhatsApp":
		return true
	default:
		return false
	}
}

func isValidStatus(v string) bool {
	switch v {
	case "Cancelado", "Em separação", "Entregue", "Faturado":
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------
// Leitura e parsing: itens_pedido.csv
// ---------------------------------------------------------------

func readItensPedidoCSV(path string) (rows []itemPedidoRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 7

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
			log.Printf("importpedidos: itens_pedido.csv linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := parseItemPedidoRow(record)
		if err != nil {
			log.Printf("importpedidos: itens_pedido.csv linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parseItemPedidoRow converte um registro CSV bruto (item_id,pedido_id,sku,
// quantidade,preco_praticado,desconto_pct,valor_bruto) em itemPedidoRow.
func parseItemPedidoRow(record []string) (itemPedidoRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	itemID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return itemPedidoRow{}, fmt.Errorf("item_id inválido (%q): %w", record[0], err)
	}

	pedidoID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return itemPedidoRow{}, fmt.Errorf("pedido_id inválido (%q): %w", record[1], err)
	}

	sku := record[2]
	if sku == "" {
		return itemPedidoRow{}, fmt.Errorf("sku vazio")
	}

	quantidade, err := strconv.Atoi(record[3])
	if err != nil {
		return itemPedidoRow{}, fmt.Errorf("quantidade inválida (%q): %w", record[3], err)
	}

	precoPraticado, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return itemPedidoRow{}, fmt.Errorf("preco_praticado inválido (%q): %w", record[4], err)
	}

	descontoPct, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return itemPedidoRow{}, fmt.Errorf("desconto_pct inválido (%q): %w", record[5], err)
	}

	valorBruto, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return itemPedidoRow{}, fmt.Errorf("valor_bruto inválido (%q): %w", record[6], err)
	}

	return itemPedidoRow{
		ItemIDOrigem:   itemID,
		PedidoIDOrigem: pedidoID,
		SKU:            sku,
		Quantidade:     quantidade,
		PrecoPraticado: precoPraticado,
		DescontoPct:    descontoPct,
		ValorBruto:     valorBruto,
	}, nil
}

// ---------------------------------------------------------------
// Lookups (pré-carregados em memória para performance, dado o volume
// de linhas de itens_pedido.csv — ~197k registros).
// ---------------------------------------------------------------

// loadClienteIDsByOrigem carrega o conjunto de cliente_id_origem existentes
// em `clientes` para lookup. Como cliente_id_origem agora É a PK da tabela
// (ver sql/09_ddl_clientes.sql), o valor usado como FK em pedidos.cliente_id
// é o próprio cliente_id_origem — o mapa é usado apenas para validar
// existência (identidade origem -> origem).
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

// loadPedidoIDsByOrigem carrega o conjunto de pedido_id_origem existentes em
// `pedidos` para lookup. Como pedido_id_origem agora É a PK da tabela (ver
// sql/04_ddl_pedidos.sql), o valor usado como FK em itens_pedido.pedido_id é
// o próprio pedido_id_origem — o mapa é usado apenas para validar existência
// (identidade origem -> origem).
func loadPedidoIDsByOrigem(db *sql.DB) (map[int64]int64, error) {
	rows, err := db.Query("SELECT pedido_id_origem FROM pedidos")
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

func loadProdutoIDsBySKU(db *sql.DB) (map[string]int64, error) {
	rows, err := db.Query("SELECT id, sku FROM produtos")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]int64)
	for rows.Next() {
		var id int64
		var sku string
		if err := rows.Scan(&id, &sku); err != nil {
			return nil, err
		}
		m[sku] = id
	}
	return m, rows.Err()
}

// ---------------------------------------------------------------
// Upserts
// ---------------------------------------------------------------

// upsertPedidos grava as linhas de pedidos no banco via INSERT ... ON
// DUPLICATE KEY UPDATE, usando pedido_id_origem como chave de idempotência.
// Linhas cujo cliente_id não é encontrado no lookup são contadas como erro
// e puladas (não abortam a importação).
func upsertPedidos(db *sql.DB, rows []pedidoRow, clienteIDs map[int64]int64) (inserted, updated, failed int) {
	const query = `
		INSERT INTO pedidos
			(pedido_id_origem, cliente_id, vendedor_id, data_pedido, canal, status, valor_total)
		VALUES
			(?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cliente_id = VALUES(cliente_id),
			vendedor_id = VALUES(vendedor_id),
			data_pedido = VALUES(data_pedido),
			canal = VALUES(canal),
			status = VALUES(status),
			valor_total = VALUES(valor_total)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importpedidos: prepare (pedidos) falhou: %v", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		clienteID, ok := clienteIDs[row.ClienteIDOrigem]
		if !ok {
			log.Printf("importpedidos: pedido_id_origem=%d: cliente_id=%d não encontrado em clientes.cliente_id_origem",
				row.PedidoIDOrigem, row.ClienteIDOrigem)
			failed++
			continue
		}

		result, err := stmt.Exec(
			row.PedidoIDOrigem, clienteID, row.VendedorID,
			row.DataPedido.Format(dataPedidoLayout), row.Canal, row.Status, row.ValorTotal,
		)
		if err != nil {
			log.Printf("importpedidos: erro no upsert de pedido_id_origem=%d: %v", row.PedidoIDOrigem, err)
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

// upsertItensPedido grava as linhas de itens de pedido no banco via
// INSERT ... ON DUPLICATE KEY UPDATE, usando item_id_origem como chave de
// idempotência. Linhas cujo pedido ou produto não são encontrados no lookup
// são contadas como erro e puladas (não abortam a importação).
func upsertItensPedido(db *sql.DB, rows []itemPedidoRow, pedidoIDs map[int64]int64, produtoIDs map[string]int64) (inserted, updated, failed int) {
	const query = `
		INSERT INTO itens_pedido
			(item_id_origem, pedido_id, produto_id, quantidade, preco_praticado, desconto_pct, valor_bruto)
		VALUES
			(?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			pedido_id = VALUES(pedido_id),
			produto_id = VALUES(produto_id),
			quantidade = VALUES(quantidade),
			preco_praticado = VALUES(preco_praticado),
			desconto_pct = VALUES(desconto_pct),
			valor_bruto = VALUES(valor_bruto)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importpedidos: prepare (itens_pedido) falhou: %v", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		pedidoID, ok := pedidoIDs[row.PedidoIDOrigem]
		if !ok {
			log.Printf("importpedidos: item_id_origem=%d: pedido_id=%d não encontrado em pedidos.pedido_id_origem",
				row.ItemIDOrigem, row.PedidoIDOrigem)
			failed++
			continue
		}

		produtoID, ok := produtoIDs[row.SKU]
		if !ok {
			log.Printf("importpedidos: item_id_origem=%d: sku=%q não encontrado em produtos.sku",
				row.ItemIDOrigem, row.SKU)
			failed++
			continue
		}

		result, err := stmt.Exec(
			row.ItemIDOrigem, pedidoID, produtoID, row.Quantidade,
			row.PrecoPraticado, row.DescontoPct, row.ValorBruto,
		)
		if err != nil {
			log.Printf("importpedidos: erro no upsert de item_id_origem=%d: %v", row.ItemIDOrigem, err)
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
// Helpers de ambiente (mesmo padrão de importclientes/importprodutos)
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
