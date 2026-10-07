// Package pagamentos implementa o importador usado por cmd/importpagamentos:
// lê dados/erp/pagamentos.csv e importa (upsert) os registros na tabela
// `pagamentos` (ver sql/12_ddl_pagamentos.sql).
//
// Uso (via comando):
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
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
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
package pagamentos

import (
	"database/sql"
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
	"github.com/rotaperfumes/shared/vlog"
)

// Tag prefixa os logs e as mensagens de erro do importador.
const Tag = "importpagamentos"

// EnvCSVPath sobrescreve o caminho default do CSV quando a flag
// -pagamentos-csv não é informada.
const EnvCSVPath = "PAGAMENTOS_CSV_PATH"

// dataLayout é o formato canônico das colunas de data em pagamentos.csv
// (data_vencimento, data_pagamento) e usado para gravação no banco.
const dataLayout = "2006-01-02"

// Row é uma linha já normalizada de pagamentos.csv, pronta para o upsert.
type Row struct {
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

// Options configura uma execução do importador.
type Options struct {
	// PagamentosCSVFlag é o valor da flag -pagamentos-csv (vazio = env
	// PAGAMENTOS_CSV_PATH ou default).
	PagamentosCSVFlag string
	// DryRun faz parsing e validação sem abrir o banco.
	DryRun bool
}

// Run executa a importação completa: resolve e lê o CSV, abre o banco via
// open (só se não for dry-run), carrega o lookup de pedidos e faz o upsert.
// Erros fatais são devolvidos sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	vlog.Printf("pagamentos.go", "Run", "declarando pagamentosCSVPath, err com resultado de ResolveCSVPath()")
	pagamentosCSVPath, err := ResolveCSVPath(opts.PagamentosCSVFlag, EnvCSVPath, "pagamentos.csv")
	vlog.Printf("pagamentos.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}

	log.Printf("importpagamentos: lendo CSV de pagamentos: %s", pagamentosCSVPath)
	vlog.Printf("pagamentos.go", "Run", "declarando rows, parseErrs, err com resultado de ReadPagamentosCSVFile()")
	rows, parseErrs, err := ReadPagamentosCSVFile(pagamentosCSVPath)
	vlog.Printf("pagamentos.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("falha ao ler CSV de pagamentos: %w", err)
	}
	log.Printf("importpagamentos: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	vlog.Printf("pagamentos.go", "Run", "verificando se opts.DryRun")
	if opts.DryRun {
		log.Printf("importpagamentos: --dry-run informado, nada foi gravado no banco")
		return nil
	}

	vlog.Printf("pagamentos.go", "Run", "declarando db, err com resultado de open()")
	db, err := open()
	vlog.Printf("pagamentos.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	vlog.Printf("pagamentos.go", "Run", "agendando defer de db.Close()")
	defer db.Close()

	vlog.Printf("pagamentos.go", "Run", "chamando db.Ping() e verificando se err != nil")
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping no banco falhou: %w", err)
	}

	vlog.Printf("pagamentos.go", "Run", "declarando pedidoIDs, err com resultado de LoadPedidoIDsByOrigem()")
	pedidoIDs, err := LoadPedidoIDsByOrigem(db)
	vlog.Printf("pagamentos.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("falha ao carregar pedidos: %w", err)
	}
	log.Printf("importpagamentos: %d pedidos carregados para lookup", len(pedidoIDs))

	vlog.Printf("pagamentos.go", "Run", "declarando inserted, updated, failed, err com resultado de UpsertPagamentos()")
	inserted, updated, failed, err := UpsertPagamentos(db, rows, pedidoIDs)
	vlog.Printf("pagamentos.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	log.Printf("importpagamentos: concluído — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
	return nil
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag > env > default (dados/erp/<fileName> na raiz do projeto).
func ResolveCSVPath(flagValue, envVar, fileName string) (string, error) {
	vlog.Printf("pagamentos.go", "ResolveCSVPath", "verificando condição do if")
	if flagValue != "" {
		return flagValue, nil
	}
	vlog.Printf("pagamentos.go", "ResolveCSVPath", "chamando os.Getenv() e verificando condição do if")
	if v := os.Getenv(envVar); v != "" {
		return v, nil
	}
	vlog.Printf("pagamentos.go", "ResolveCSVPath", "declarando root, err com resultado de cmdutil.FindProjectRoot()")
	root, err := cmdutil.FindProjectRoot()
	vlog.Printf("pagamentos.go", "ResolveCSVPath", "verificando se err != nil")
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a raiz do projeto: %w", err)
	}
	return filepath.Join(root, "dados", "erp", fileName), nil
}

// ---------------------------------------------------------------
// Leitura e parsing: pagamentos.csv
// ---------------------------------------------------------------

// ReadPagamentosCSVFile abre o arquivo em path e delega para ReadPagamentosCSV.
func ReadPagamentosCSVFile(path string) (rows []Row, parseErrs int, err error) {
	vlog.Printf("pagamentos.go", "ReadPagamentosCSVFile", "declarando f, err com resultado de os.Open()")
	f, err := os.Open(path)
	vlog.Printf("pagamentos.go", "ReadPagamentosCSVFile", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	vlog.Printf("pagamentos.go", "ReadPagamentosCSVFile", "agendando defer de f.Close()")
	defer f.Close()
	return ReadPagamentosCSV(f)
}

// ReadPagamentosCSV lê e normaliza pagamentos.csv (com cabeçalho). Linhas
// malformadas são contadas em parseErrs e puladas.
func ReadPagamentosCSV(rd io.Reader) (rows []Row, parseErrs int, err error) {
	vlog.Printf("pagamentos.go", "ReadPagamentosCSV", "declarando r com resultado de csv.NewReader()")
	r := csv.NewReader(rd)
	vlog.Printf("pagamentos.go", "ReadPagamentosCSV", "atribuindo a r.FieldsPerRecord o valor de valor literal")
	r.FieldsPerRecord = 10

	vlog.Printf("pagamentos.go", "ReadPagamentosCSV", "chamando r.Read() e verificando se err != nil")
	if _, err := r.Read(); err != nil {
		return nil, 0, fmt.Errorf("lendo cabeçalho: %w", err)
	}

	vlog.Printf("pagamentos.go", "ReadPagamentosCSV", "declarando lineNum com valor literal")
	lineNum := 1
	vlog.Printf("pagamentos.go", "ReadPagamentosCSV", "iniciando loop for sem condição (até break)")
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

		row, err := ParsePagamentoRow(record)
		if err != nil {
			log.Printf("importpagamentos: pagamentos.csv linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	vlog.Printf("pagamentos.go", "ReadPagamentosCSV", "loop concluído; linhas válidas: %d, erros de parsing: %d", len(rows), parseErrs)
	return rows, parseErrs, nil
}

// ParsePagamentoRow converte um registro CSV bruto (pagamento_id,pedido_id,
// forma_pagamento,parcelas,valor,taxa_pct,valor_liquido,data_vencimento,
// data_pagamento,status_pagamento) em Row.
func ParsePagamentoRow(record []string) (Row, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	pagamentoID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("pagamento_id inválido (%q): %w", record[0], err)
	}

	pedidoID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("pedido_id inválido (%q): %w", record[1], err)
	}

	formaPagamento := record[2]
	if !IsValidFormaPagamento(formaPagamento) {
		return Row{}, fmt.Errorf("forma_pagamento inválida (%q)", formaPagamento)
	}

	parcelas, err := strconv.Atoi(record[3])
	if err != nil {
		return Row{}, fmt.Errorf("parcelas inválido (%q): %w", record[3], err)
	}

	valor, err := strconv.ParseFloat(record[4], 64)
	if err != nil {
		return Row{}, fmt.Errorf("valor inválido (%q): %w", record[4], err)
	}

	taxaPct, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return Row{}, fmt.Errorf("taxa_pct inválido (%q): %w", record[5], err)
	}

	valorLiquido, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return Row{}, fmt.Errorf("valor_liquido inválido (%q): %w", record[6], err)
	}

	dataVencimento, err := ParseData(record[7])
	if err != nil {
		return Row{}, fmt.Errorf("data_vencimento inválida (%q): %w", record[7], err)
	}

	var dataPagamento sql.NullString
	if record[8] != "" {
		d, err := ParseData(record[8])
		if err != nil {
			return Row{}, fmt.Errorf("data_pagamento inválida (%q): %w", record[8], err)
		}
		dataPagamento = sql.NullString{String: d, Valid: true}
	}

	statusPagamento := record[9]
	if !IsValidStatusPagamento(statusPagamento) {
		return Row{}, fmt.Errorf("status_pagamento inválido (%q)", statusPagamento)
	}

	return Row{
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

// ParseData valida e normaliza uma data no layout ISO (2006-01-02).
func ParseData(v string) (string, error) {
	t, err := time.ParseInLocation(dataLayout, v, time.Local)
	if err != nil {
		return "", err
	}
	return t.Format(dataLayout), nil
}

// IsValidFormaPagamento reporta se v é uma forma_pagamento aceita (ENUM da tabela).
func IsValidFormaPagamento(v string) bool {
	switch v {
	case "Boleto 14 dias", "Boleto 28 dias", "Cartão de crédito", "Cartão de débito", "Cheque a prazo", "Dinheiro", "PIX":
		return true
	default:
		return false
	}
}

// IsValidStatusPagamento reporta se v é um status_pagamento aceito (ENUM da tabela).
func IsValidStatusPagamento(v string) bool {
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

// LoadPedidoIDsByOrigem pré-carrega o conjunto de pedido_id_origem
// existentes em `pedidos`, usado para validar o pedido_id do CSV. Como
// pedido_id_origem agora É a PK da tabela (ver sql/04_ddl_pedidos.sql), o
// valor usado como FK em pagamentos.pedido_id é o próprio pedido_id_origem
// — o mapa serve apenas para checar existência (identidade origem -> origem).
func LoadPedidoIDsByOrigem(db cmdutil.DB) (map[int64]int64, error) {
	vlog.Printf("pagamentos.go", "LoadPedidoIDsByOrigem", "declarando rows, err com resultado de db.Query()")
	rows, err := db.Query("SELECT pedido_id_origem FROM pedidos")
	vlog.Printf("pagamentos.go", "LoadPedidoIDsByOrigem", "verificando se err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("pagamentos.go", "LoadPedidoIDsByOrigem", "agendando defer de rows.Close()")
	defer rows.Close()

	vlog.Printf("pagamentos.go", "LoadPedidoIDsByOrigem", "declarando m com resultado de make()")
	m := make(map[int64]int64)
	vlog.Printf("pagamentos.go", "LoadPedidoIDsByOrigem", "iniciando loop for enquanto rows.Next()")
	for rows.Next() {
		var origem int64
		if err := rows.Scan(&origem); err != nil {
			return nil, err
		}
		m[origem] = origem
	}
	vlog.Printf("pagamentos.go", "LoadPedidoIDsByOrigem", "loop concluído; registros carregados: %d", len(m))
	return m, rows.Err()
}

// ---------------------------------------------------------------
// Upsert
// ---------------------------------------------------------------

// UpsertPagamentos grava as linhas de pagamentos no banco via INSERT ... ON
// DUPLICATE KEY UPDATE, usando o próprio `pagamento_id` (PK, vindo do CSV)
// como chave de idempotência — diferente de pedidos/itens_pedido, aqui NÃO
// há coluna `id` desacoplada nem `pagamento_id_origem` (ver
// sql/12_ddl_pagamentos.sql). Linhas cujo pedido_id não é encontrado no
// lookup são contadas como erro e puladas (não abortam a importação). Só
// devolve err se o prepare falhar.
func UpsertPagamentos(db cmdutil.DB, rows []Row, pedidoIDs map[int64]int64) (inserted, updated, failed int, err error) {
	vlog.Printf("pagamentos.go", "UpsertPagamentos", "declarando constante query")
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

	vlog.Printf("pagamentos.go", "UpsertPagamentos", "declarando stmt, err com resultado de db.Prepare()")
	stmt, err := db.Prepare(query)
	vlog.Printf("pagamentos.go", "UpsertPagamentos", "verificando se err != nil")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("prepare (pagamentos) falhou: %w", err)
	}
	vlog.Printf("pagamentos.go", "UpsertPagamentos", "agendando defer de stmt.Close()")
	defer stmt.Close()

	vlog.Printf("pagamentos.go", "UpsertPagamentos", "iniciando loop range sobre rows")
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
	vlog.Printf("pagamentos.go", "UpsertPagamentos", "loop concluído; itens: %d", len(rows))
	return inserted, updated, failed, nil
}
