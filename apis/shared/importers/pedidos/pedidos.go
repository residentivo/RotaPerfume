// Package pedidos implementa o importador usado por cmd/importpedidos: lê
// dados/erp/pedidos.csv e dados/erp/itens_pedido.csv e importa (upsert) os
// registros nas tabelas `pedidos` e `itens_pedido` (ver
// sql/04_ddl_pedidos.sql e sql/11_ddl_itens_pedido.sql), nessa ordem (itens
// dependem dos pedidos já importados).
//
// Uso (via comando):
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
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
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
package pedidos

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/clientesdedup"
)

// Tag prefixa os logs e as mensagens de erro do importador.
const Tag = "importpedidos"

// Variáveis de ambiente que sobrescrevem os caminhos default dos CSVs quando
// as flags -pedidos-csv / -itens-csv não são informadas.
const (
	EnvPedidosCSVPath = "PEDIDOS_CSV_PATH"
	EnvItensCSVPath   = "ITENS_PEDIDO_CSV_PATH"
)

// dataPedidoLayout é o formato canônico usado para gravar data_pedido no banco.
const dataPedidoLayout = "2006-01-02"

// dataPedidoLayoutBR é um formato alternativo (DD/MM/YYYY) observado em parte
// das linhas de pedidos.csv. O parser tenta ISO primeiro e, se falhar, tenta
// este formato antes de contar a linha como erro de parsing.
const dataPedidoLayoutBR = "02/01/2006"

// PedidoRow é uma linha já normalizada de pedidos.csv, pronta para o upsert.
type PedidoRow struct {
	PedidoIDOrigem  int64
	ClienteIDOrigem int64
	VendedorID      int64
	DataPedido      time.Time
	Canal           string
	Status          string
	ValorTotal      float64
}

// ItemPedidoRow é uma linha já normalizada de itens_pedido.csv, pronta para o upsert.
type ItemPedidoRow struct {
	ItemIDOrigem   int64
	PedidoIDOrigem int64
	SKU            string
	Quantidade     int
	PrecoPraticado float64
	DescontoPct    float64
	ValorBruto     float64
}

// Options configura uma execução do importador.
type Options struct {
	// PedidosCSVFlag é o valor da flag -pedidos-csv (vazio = env PEDIDOS_CSV_PATH ou default).
	PedidosCSVFlag string
	// ItensCSVFlag é o valor da flag -itens-csv (vazio = env ITENS_PEDIDO_CSV_PATH ou default).
	ItensCSVFlag string
	// DryRun faz parsing e validação dos dois CSVs sem abrir o banco.
	DryRun bool
}

// Run executa a importação completa nas duas fases (pedidos e itens),
// abrindo o banco via open só se não for dry-run. Erros fatais são
// devolvidos sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	pedidosCSVPath, err := ResolveCSVPath(opts.PedidosCSVFlag, EnvPedidosCSVPath, "pedidos.csv")
	if err != nil {
		return err
	}
	itensCSVPath, err := ResolveCSVPath(opts.ItensCSVFlag, EnvItensCSVPath, "itens_pedido.csv")
	if err != nil {
		return err
	}

	// -----------------------------------------------------------
	// FASE 1: pedidos.csv
	// -----------------------------------------------------------
	log.Printf("importpedidos: [fase 1/2] lendo CSV de pedidos: %s", pedidosCSVPath)
	pedidoRows, pedidoParseErrs, err := ReadPedidosCSVFile(pedidosCSVPath)
	if err != nil {
		return fmt.Errorf("falha ao ler CSV de pedidos: %w", err)
	}
	log.Printf("importpedidos: [fase 1/2] %d linhas válidas lidas, %d linhas com erro de parsing", len(pedidoRows), pedidoParseErrs)

	if opts.DryRun {
		log.Printf("importpedidos: [fase 2/2] lendo CSV de itens de pedido: %s", itensCSVPath)
		itemRows, itemParseErrs, err := ReadItensPedidoCSVFile(itensCSVPath)
		if err != nil {
			return fmt.Errorf("falha ao ler CSV de itens de pedido: %w", err)
		}
		log.Printf("importpedidos: [fase 2/2] %d linhas válidas lidas, %d linhas com erro de parsing", len(itemRows), itemParseErrs)
		log.Printf("importpedidos: --dry-run informado, nada foi gravado no banco")
		return nil
	}

	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping no banco falhou: %w", err)
	}

	clienteIDs, err := LoadClienteIDsByOrigem(db)
	if err != nil {
		return fmt.Errorf("falha ao carregar clientes: %w", err)
	}
	log.Printf("importpedidos: %d clientes carregados para lookup", len(clienteIDs))
	// NEG-01: cópias de CNPJ duplicado no clientes.csv apontam para o sobrevivente.
	projectRoot, _ := cmdutil.FindProjectRoot()
	clientesdedup.Redirecionar(Tag, projectRoot, clienteIDs)

	pInserted, pUpdated, pFailed, err := UpsertPedidos(db, pedidoRows, clienteIDs)
	if err != nil {
		return err
	}
	log.Printf("importpedidos: [fase 1/2] OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(pedidoRows), pInserted, pUpdated, pFailed, pedidoParseErrs)

	// -----------------------------------------------------------
	// FASE 2: itens_pedido.csv
	// -----------------------------------------------------------
	log.Printf("importpedidos: [fase 2/2] lendo CSV de itens de pedido: %s", itensCSVPath)
	itemRows, itemParseErrs, err := ReadItensPedidoCSVFile(itensCSVPath)
	if err != nil {
		return fmt.Errorf("falha ao ler CSV de itens de pedido: %w", err)
	}
	log.Printf("importpedidos: [fase 2/2] %d linhas válidas lidas, %d linhas com erro de parsing", len(itemRows), itemParseErrs)

	pedidoIDs, err := LoadPedidoIDsByOrigem(db)
	if err != nil {
		return fmt.Errorf("falha ao carregar pedidos: %w", err)
	}
	log.Printf("importpedidos: %d pedidos carregados para lookup", len(pedidoIDs))

	produtoIDs, err := LoadProdutoIDsBySKU(db)
	if err != nil {
		return fmt.Errorf("falha ao carregar produtos: %w", err)
	}
	log.Printf("importpedidos: %d produtos carregados para lookup", len(produtoIDs))

	iInserted, iUpdated, iFailed, err := UpsertItensPedido(db, itemRows, pedidoIDs, produtoIDs)
	if err != nil {
		return err
	}
	log.Printf("importpedidos: [fase 2/2] OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(itemRows), iInserted, iUpdated, iFailed, itemParseErrs)

	log.Printf("importpedidos: concluído — pedidos(inseridos_ou_inalterados=%d atualizados=%d erros=%d) itens(inseridos_ou_inalterados=%d atualizados=%d erros=%d)",
		pInserted, pUpdated, pFailed, iInserted, iUpdated, iFailed)
	return nil
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag > env > default (dados/erp/<fileName> na raiz do projeto).
func ResolveCSVPath(flagValue, envVar, fileName string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	root, err := cmdutil.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a raiz do projeto: %w", err)
	}
	return filepath.Join(root, "dados", "erp", fileName), nil
}

// ---------------------------------------------------------------
// Leitura e parsing: pedidos.csv
// ---------------------------------------------------------------

// ReadPedidosCSVFile abre o arquivo em path e delega para ReadPedidosCSV.
func ReadPedidosCSVFile(path string) (rows []PedidoRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()
	return ReadPedidosCSV(f)
}

// ReadPedidosCSV lê e normaliza pedidos.csv (com cabeçalho). Linhas
// malformadas são contadas em parseErrs e puladas.
func ReadPedidosCSV(rd io.Reader) (rows []PedidoRow, parseErrs int, err error) {
	r := csv.NewReader(rd)
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

		row, err := ParsePedidoRow(record)
		if err != nil {
			log.Printf("importpedidos: pedidos.csv linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// ParsePedidoRow converte um registro CSV bruto (pedido_id,cliente_id,
// vendedor_id,data_pedido,canal,status,valor_total) em PedidoRow.
func ParsePedidoRow(record []string) (PedidoRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	pedidoID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return PedidoRow{}, fmt.Errorf("pedido_id inválido (%q): %w", record[0], err)
	}

	clienteID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return PedidoRow{}, fmt.Errorf("cliente_id inválido (%q): %w", record[1], err)
	}

	vendedorID, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return PedidoRow{}, fmt.Errorf("vendedor_id inválido (%q): %w", record[2], err)
	}

	dataPedido, err := ParseDataPedido(record[3])
	if err != nil {
		return PedidoRow{}, fmt.Errorf("data_pedido inválida (%q): %w", record[3], err)
	}

	canal := record[4]
	if !IsValidCanal(canal) {
		return PedidoRow{}, fmt.Errorf("canal inválido (%q)", canal)
	}

	status := record[5]
	if !IsValidStatus(status) {
		return PedidoRow{}, fmt.Errorf("status inválido (%q)", status)
	}

	valorTotal, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return PedidoRow{}, fmt.Errorf("valor_total inválido (%q): %w", record[6], err)
	}

	return PedidoRow{
		PedidoIDOrigem:  pedidoID,
		ClienteIDOrigem: clienteID,
		VendedorID:      vendedorID,
		DataPedido:      dataPedido,
		Canal:           canal,
		Status:          status,
		ValorTotal:      valorTotal,
	}, nil
}

// ParseDataPedido tenta o layout ISO (2006-01-02) primeiro e, se falhar,
// tenta o layout BR (02/01/2006), pois pedidos.csv mistura ambos os formatos.
// Retorna erro se nenhum dos dois formatos casar.
func ParseDataPedido(v string) (time.Time, error) {
	if t, err := time.ParseInLocation(dataPedidoLayout, v, time.Local); err == nil {
		return t, nil
	}
	return time.ParseInLocation(dataPedidoLayoutBR, v, time.Local)
}

// IsValidCanal reporta se v é um canal de pedido aceito (ENUM da tabela).
func IsValidCanal(v string) bool {
	switch v {
	case "App", "Telefone", "Visita", "WhatsApp":
		return true
	default:
		return false
	}
}

// IsValidStatus reporta se v é um status de pedido aceito (ENUM da tabela).
func IsValidStatus(v string) bool {
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

// ReadItensPedidoCSVFile abre o arquivo em path e delega para ReadItensPedidoCSV.
func ReadItensPedidoCSVFile(path string) (rows []ItemPedidoRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()
	return ReadItensPedidoCSV(f)
}

// ReadItensPedidoCSV lê e normaliza itens_pedido.csv (com cabeçalho). Linhas
// malformadas são contadas em parseErrs e puladas.
func ReadItensPedidoCSV(rd io.Reader) (rows []ItemPedidoRow, parseErrs int, err error) {
	r := csv.NewReader(rd)
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

		row, err := ParseItemPedidoRow(record)
		if err != nil {
			log.Printf("importpedidos: itens_pedido.csv linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// ParseItemPedidoRow converte um registro CSV bruto (item_id,pedido_id,sku,
// quantidade,preco_praticado,desconto_pct,valor_bruto) em ItemPedidoRow.
func ParseItemPedidoRow(record []string) (ItemPedidoRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	itemID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return ItemPedidoRow{}, fmt.Errorf("item_id inválido (%q): %w", record[0], err)
	}

	pedidoID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return ItemPedidoRow{}, fmt.Errorf("pedido_id inválido (%q): %w", record[1], err)
	}

	sku := record[2]
	if sku == "" {
		return ItemPedidoRow{}, fmt.Errorf("sku vazio")
	}

	quantidade, err := strconv.Atoi(record[3])
	if err != nil {
		return ItemPedidoRow{}, fmt.Errorf("quantidade inválida (%q): %w", record[3], err)
	}

	precoPraticado, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return ItemPedidoRow{}, fmt.Errorf("preco_praticado inválido (%q): %w", record[4], err)
	}

	descontoPct, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return ItemPedidoRow{}, fmt.Errorf("desconto_pct inválido (%q): %w", record[5], err)
	}

	valorBruto, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return ItemPedidoRow{}, fmt.Errorf("valor_bruto inválido (%q): %w", record[6], err)
	}

	return ItemPedidoRow{
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

// LoadClienteIDsByOrigem carrega o conjunto de cliente_id_origem existentes
// em `clientes` para lookup. Como cliente_id_origem agora É a PK da tabela
// (ver sql/09_ddl_clientes.sql), o valor usado como FK em pedidos.cliente_id
// é o próprio cliente_id_origem — o mapa é usado apenas para validar
// existência (identidade origem -> origem).
func LoadClienteIDsByOrigem(db cmdutil.DB) (map[int64]int64, error) {
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

// LoadPedidoIDsByOrigem carrega o conjunto de pedido_id_origem existentes em
// `pedidos` para lookup. Como pedido_id_origem agora É a PK da tabela (ver
// sql/04_ddl_pedidos.sql), o valor usado como FK em itens_pedido.pedido_id é
// o próprio pedido_id_origem — o mapa é usado apenas para validar existência
// (identidade origem -> origem).
func LoadPedidoIDsByOrigem(db cmdutil.DB) (map[int64]int64, error) {
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

// LoadProdutoIDsBySKU carrega sku → produtos.id para o lookup dos itens.
func LoadProdutoIDsBySKU(db cmdutil.DB) (map[string]int64, error) {
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

// UpsertPedidos grava as linhas de pedidos no banco via INSERT ... ON
// DUPLICATE KEY UPDATE, usando pedido_id_origem como chave de idempotência.
// Linhas cujo cliente_id não é encontrado no lookup são contadas como erro
// e puladas (não abortam a importação). Só devolve err se
// o prepare falhar.
func UpsertPedidos(db cmdutil.DB, rows []PedidoRow, clienteIDs map[int64]int64) (inserted, updated, failed int, err error) {
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
		return 0, 0, 0, fmt.Errorf("prepare (pedidos) falhou: %w", err)
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
			updated++
		}
	}
	return inserted, updated, failed, nil
}

// UpsertItensPedido grava as linhas de itens de pedido no banco via
// INSERT ... ON DUPLICATE KEY UPDATE, usando item_id_origem como chave de
// idempotência. Linhas cujo pedido ou produto não são encontrados no lookup
// são contadas como erro e puladas (não abortam a importação). Só devolve err se
// o prepare falhar.
func UpsertItensPedido(db cmdutil.DB, rows []ItemPedidoRow, pedidoIDs map[int64]int64, produtoIDs map[string]int64) (inserted, updated, failed int, err error) {
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
		return 0, 0, 0, fmt.Errorf("prepare (itens_pedido) falhou: %w", err)
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
			updated++
		}
	}
	return inserted, updated, failed, nil
}
