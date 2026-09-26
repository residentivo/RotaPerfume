// Package produtos implementa o importador usado por cmd/importprodutos: lê
// dados/erp/produtos.csv e importa (upsert) os registros na tabela
// `produtos` (ver sql/10_ddl_produtos.sql).
//
// Uso (via comando):
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
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
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
//     atualizadas (update), ignoradas e com erro — e devolve erro (o comando
//     encerra com log.Fatalf) se houver erro de conexão ou de leitura do arquivo.
package produtos

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
)

// Tag prefixa os logs e as mensagens de erro do importador.
const Tag = "importprodutos"

// EnvCSVPath sobrescreve o caminho default do CSV quando a flag -csv não é informada.
const EnvCSVPath = "PRODUTOS_CSV_PATH"

// dataLancamentoLayout é o formato de data_lancamento aceito no CSV.
const dataLancamentoLayout = "2006-01-02"

// Row é uma linha já normalizada do CSV, pronta para o upsert.
type Row struct {
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

// Options configura uma execução do importador.
type Options struct {
	// CSVFlag é o valor da flag -csv (vazio = env PRODUTOS_CSV_PATH ou default).
	CSVFlag string
	// DryRun faz parsing e validação sem abrir o banco.
	DryRun bool
}

// Run executa a importação completa: resolve e lê o CSV, abre o banco via
// open (só se não for dry-run) e faz o upsert. Erros fatais são devolvidos
// sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	csvPath, err := ResolveCSVPath(opts.CSVFlag)
	if err != nil {
		return err
	}
	log.Printf("importprodutos: lendo CSV de %s", csvPath)

	rows, parseErrs, err := ReadCSVFile(csvPath)
	if err != nil {
		return fmt.Errorf("falha ao ler CSV: %w", err)
	}
	log.Printf("importprodutos: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	if opts.DryRun {
		log.Printf("importprodutos: --dry-run informado, nada foi gravado no banco")
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

	inserted, updated, failed, err := UpsertAll(db, rows)
	if err != nil {
		return err
	}

	log.Printf("importprodutos: OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
	return nil
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env PRODUTOS_CSV_PATH > default (dados/erp/produtos.csv na raiz do projeto).
func ResolveCSVPath(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if v := os.Getenv(EnvCSVPath); v != "" {
		return v, nil
	}
	root, err := cmdutil.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a raiz do projeto: %w", err)
	}
	return filepath.Join(root, "dados", "erp", "produtos.csv"), nil
}

// ReadCSVFile abre o arquivo em path e delega para ReadCSV.
func ReadCSVFile(path string) (rows []Row, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()
	return ReadCSV(f)
}

// ReadCSV lê e normaliza o CSV (com cabeçalho). Linhas malformadas são
// contadas em parseErrs e puladas (não abortam a importação inteira).
func ReadCSV(rd io.Reader) (rows []Row, parseErrs int, err error) {
	r := csv.NewReader(rd)
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

		row, err := ParseRow(record)
		if err != nil {
			log.Printf("importprodutos: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// ParseRow converte um registro CSV bruto em Row, aplicando TRIM,
// parsing de floats, conversão de 'S'/'N' para bool (default TRUE quando
// vazio/inválido) e parsing opcional de data_lancamento.
func ParseRow(record []string) (Row, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	sku := record[0]
	descricao := record[1]
	categoria := record[2]
	marca := record[3]
	notaOlfativa := record[4]

	if sku == "" {
		return Row{}, fmt.Errorf("sku vazio")
	}
	if descricao == "" || categoria == "" || marca == "" {
		return Row{}, fmt.Errorf("descricao, categoria ou marca vazios")
	}

	precoTabela, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return Row{}, fmt.Errorf("preco_tabela inválido (%q): %w", record[5], err)
	}

	custoUnitario, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return Row{}, fmt.Errorf("custo_unitario inválido (%q): %w", record[6], err)
	}

	unidade := record[7]
	if unidade == "" {
		return Row{}, fmt.Errorf("unidade vazia")
	}

	ativo := ParseAtivo(record[8])

	dataLancamento, err := ParseDataLancamento(record[9])
	if err != nil {
		return Row{}, fmt.Errorf("data_lancamento inválida (%q): %w", record[9], err)
	}

	return Row{
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

// ParseAtivo converte 'S'/'N' (case-insensitive) para bool. REGRA DE
// NEGÓCIO: qualquer valor vazio ou não reconhecido é tratado como ATIVO
// (default TRUE) — o padrão exigido para itens importados. Só fica inativo
// quando o CSV traz 'N' explicitamente.
func ParseAtivo(raw string) bool {
	switch strings.ToUpper(raw) {
	case "N":
		return false
	default:
		return true
	}
}

// ParseDataLancamento faz parsing opcional de data_lancamento. Vazio = NULL (nil).
func ParseDataLancamento(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation(dataLancamentoLayout, raw, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY UPDATE,
// usando sku como chave de idempotência. Só devolve err se o prepare falhar.
func UpsertAll(db cmdutil.DB, rows []Row) (inserted, updated, failed int, err error) {
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
		return 0, 0, 0, fmt.Errorf("prepare falhou: %w", err)
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
	return inserted, updated, failed, nil
}
