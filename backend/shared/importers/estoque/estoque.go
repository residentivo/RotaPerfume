// Package estoque implementa o importador usado por cmd/importestoque: lê
// dados/erp/estoque.csv e importa (upsert) os registros na tabela `estoque`
// (ver sql/17_ddl_estoque.sql).
//
// Uso (via comando):
//
//	cd backend/shared && go run ./cmd/importestoque
//	cd backend/shared && go run ./cmd/importestoque -csv=/caminho/alternativo/estoque.csv
//	cd backend/shared && go run ./cmd/importestoque -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-estoque
//
// Comportamento:
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
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
//     atualizadas (update), ignoradas e com erro — e devolve erro (o comando
//     encerra com log.Fatalf) se houver erro de conexão ou de leitura do arquivo.
package estoque

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/importkit"
)

// Tag prefixa os logs e as mensagens de erro do importador.
const Tag = "importestoque"

// EnvCSVPath sobrescreve o caminho default do CSV quando a flag -csv não é informada.
const EnvCSVPath = "ESTOQUE_CSV_PATH"

// dataSnapshotLayout é o formato de data_snapshot aceito no CSV.
const dataSnapshotLayout = "2006-01-02"

// Row é uma linha já normalizada do CSV, pronta para o upsert.
type Row struct {
	DataSnapshot time.Time
	SKU          string
	Saldo        int
	Ruptura      bool
}

// Options configura uma execução do importador.
type Options struct {
	// CSVFlag é o valor da flag -csv (vazio = env ESTOQUE_CSV_PATH ou default).
	CSVFlag string
	// DryRun faz parsing e validação sem abrir o banco.
	DryRun bool
}

// Run executa a importação completa: resolve e lê o CSV, abre o banco via
// open (só se não for dry-run) e faz o upsert. Erros fatais são devolvidos
// sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	i := &importacao{opts: opts}
	return importkit.Executar(importkit.Etapas{Tag: Tag, Ler: i.ler, Gravar: i.gravar}, opts.DryRun, open)
}

// importacao guarda o estado de uma execução entre os passos do pipeline.
type importacao struct {
	opts      Options
	rows      []Row
	invalidas int
}

func (i *importacao) ler() error {
	csvPath, err := ResolveCSVPath(i.opts.CSVFlag)
	if err != nil {
		return err
	}
	log.Printf("importestoque: lendo CSV de %s", csvPath)
	if i.rows, i.invalidas, err = ReadCSVFile(csvPath); err != nil {
		return fmt.Errorf("falha ao ler CSV: %w", err)
	}
	log.Printf("importestoque: %d linhas válidas lidas, %d linhas com erro de parsing", len(i.rows), i.invalidas)
	return nil
}

func (i *importacao) gravar(db cmdutil.DB) error {
	inserted, updated, failed, err := UpsertAll(db, i.rows)
	if err != nil {
		return err
	}
	log.Printf("importestoque: OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(i.rows), inserted, updated, failed, i.invalidas)
	return nil
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env ESTOQUE_CSV_PATH > default (dados/erp/estoque.csv na raiz do projeto).
func ResolveCSVPath(flagValue string) (string, error) {
	return importkit.ResolverCSV(flagValue, EnvCSVPath, "dados", "erp", "estoque.csv")
}

// ReadCSVFile abre o arquivo em path e delega para ReadCSV.
func ReadCSVFile(path string) (rows []Row, parseErrs int, err error) {
	return importkit.LerArquivo(path, ReadCSV)
}

// ReadCSV lê e normaliza o CSV (com cabeçalho). Linhas malformadas são
// contadas em parseErrs e puladas (não abortam a importação inteira).
func ReadCSV(rd io.Reader) (rows []Row, parseErrs int, err error) {
	return importkit.LerCSV(rd, importkit.Formato{Prefixo: "importestoque:", Campos: 4}, ParseRow)
}

// ParseRow converte um registro CSV bruto em Row, aplicando TRIM,
// parsing de data e inteiro, e conversão de 'S'/'N' para bool (default
// FALSE quando vazio/inválido).
func ParseRow(record []string) (Row, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	dataSnapshotRaw := record[0]
	sku := record[1]

	if sku == "" {
		return Row{}, fmt.Errorf("sku vazio")
	}

	dataSnapshot, err := time.ParseInLocation(dataSnapshotLayout, dataSnapshotRaw, time.Local)
	if err != nil {
		return Row{}, fmt.Errorf("data_snapshot inválida (%q): %w", dataSnapshotRaw, err)
	}

	saldo, err := strconv.Atoi(record[2])
	if err != nil {
		return Row{}, fmt.Errorf("saldo inválido (%q): %w", record[2], err)
	}

	ruptura := ParseRuptura(record[3])

	return Row{
		DataSnapshot: dataSnapshot,
		SKU:          sku,
		Saldo:        saldo,
		Ruptura:      ruptura,
	}, nil
}

// ParseRuptura converte 'S'/'N' (case-insensitive) para bool. Qualquer
// valor vazio ou não reconhecido é tratado como SEM RUPTURA (default
// FALSE) — só fica em ruptura quando o CSV traz 'S' explicitamente.
func ParseRuptura(raw string) bool {
	return strings.ToUpper(raw) == "S"
}

// UpsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY UPDATE,
// usando a chave composta (data_snapshot, sku) como idempotência. O saldo do
// CSV do ERP é sempre a fonte de verdade para o saldo absoluto do dia,
// mesmo que uma baixa por faturamento tenha gravado esse par
// (data_snapshot, sku) antes. Só devolve err se o prepare falhar.
func UpsertAll(db cmdutil.DB, rows []Row) (inserted, updated, failed int, err error) {
	c, err := importkit.Upsert[Row]{
		SQL: `
		INSERT INTO estoque
			(data_snapshot, sku, saldo, ruptura)
		VALUES
			(?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			saldo = VALUES(saldo),
			ruptura = VALUES(ruptura)
	`,
		ErroPrepare: "prepare falhou",
		Args: func(row Row) ([]any, bool) {
			return []any{row.DataSnapshot.Format(dataSnapshotLayout), row.SKU, row.Saldo, row.Ruptura}, true
		},
		LogFalha: func(row Row, err error) {
			log.Printf("importestoque: erro no upsert de data_snapshot=%s sku=%s: %v",
				row.DataSnapshot.Format(dataSnapshotLayout), row.SKU, err)
		},
	}.Gravar(db, rows)
	return c.Inseridos, c.Atualizados, c.Falhas, err
}
