// Package estoque implementa o importador usado por cmd/importestoque: lê
// dados/erp/estoque.csv e importa (upsert) os registros na tabela `estoque`
// (ver sql/17_ddl_estoque.sql).
//
// Uso (via comando):
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
	vlog.Printf("estoque.go", "Run", "declarando csvPath, err com resultado de ResolveCSVPath()")
	csvPath, err := ResolveCSVPath(opts.CSVFlag)
	vlog.Printf("estoque.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	log.Printf("importestoque: lendo CSV de %s", csvPath)

	vlog.Printf("estoque.go", "Run", "declarando rows, parseErrs, err com resultado de ReadCSVFile()")
	rows, parseErrs, err := ReadCSVFile(csvPath)
	vlog.Printf("estoque.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("falha ao ler CSV: %w", err)
	}
	log.Printf("importestoque: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	vlog.Printf("estoque.go", "Run", "verificando se opts.DryRun")
	if opts.DryRun {
		log.Printf("importestoque: --dry-run informado, nada foi gravado no banco")
		return nil
	}

	vlog.Printf("estoque.go", "Run", "declarando db, err com resultado de open()")
	db, err := open()
	vlog.Printf("estoque.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	vlog.Printf("estoque.go", "Run", "agendando defer de db.Close()")
	defer db.Close()

	vlog.Printf("estoque.go", "Run", "chamando db.Ping() e verificando se err != nil")
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping no banco falhou: %w", err)
	}

	vlog.Printf("estoque.go", "Run", "declarando inserted, updated, failed, err com resultado de UpsertAll()")
	inserted, updated, failed, err := UpsertAll(db, rows)
	vlog.Printf("estoque.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}

	log.Printf("importestoque: OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
	return nil
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env ESTOQUE_CSV_PATH > default (dados/erp/estoque.csv na raiz do projeto).
func ResolveCSVPath(flagValue string) (string, error) {
	vlog.Printf("estoque.go", "ResolveCSVPath", "verificando condição do if")
	if flagValue != "" {
		return flagValue, nil
	}
	vlog.Printf("estoque.go", "ResolveCSVPath", "chamando os.Getenv() e verificando condição do if")
	if v := os.Getenv(EnvCSVPath); v != "" {
		return v, nil
	}
	vlog.Printf("estoque.go", "ResolveCSVPath", "declarando root, err com resultado de cmdutil.FindProjectRoot()")
	root, err := cmdutil.FindProjectRoot()
	vlog.Printf("estoque.go", "ResolveCSVPath", "verificando se err != nil")
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a raiz do projeto: %w", err)
	}
	return filepath.Join(root, "dados", "erp", "estoque.csv"), nil
}

// ReadCSVFile abre o arquivo em path e delega para ReadCSV.
func ReadCSVFile(path string) (rows []Row, parseErrs int, err error) {
	vlog.Printf("estoque.go", "ReadCSVFile", "declarando f, err com resultado de os.Open()")
	f, err := os.Open(path)
	vlog.Printf("estoque.go", "ReadCSVFile", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	vlog.Printf("estoque.go", "ReadCSVFile", "agendando defer de f.Close()")
	defer f.Close()
	return ReadCSV(f)
}

// ReadCSV lê e normaliza o CSV (com cabeçalho). Linhas malformadas são
// contadas em parseErrs e puladas (não abortam a importação inteira).
func ReadCSV(rd io.Reader) (rows []Row, parseErrs int, err error) {
	vlog.Printf("estoque.go", "ReadCSV", "declarando r com resultado de csv.NewReader()")
	r := csv.NewReader(rd)
	vlog.Printf("estoque.go", "ReadCSV", "atribuindo a r.FieldsPerRecord o valor de valor literal")
	r.FieldsPerRecord = 4

	// Descarta o cabeçalho.
	vlog.Printf("estoque.go", "ReadCSV", "chamando r.Read() e verificando se err != nil")
	if _, err := r.Read(); err != nil {
		return nil, 0, fmt.Errorf("lendo cabeçalho: %w", err)
	}

	vlog.Printf("estoque.go", "ReadCSV", "declarando lineNum com valor literal")
	lineNum := 1
	vlog.Printf("estoque.go", "ReadCSV", "iniciando loop for sem condição (até break)")
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

		row, err := ParseRow(record)
		if err != nil {
			log.Printf("importestoque: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	vlog.Printf("estoque.go", "ReadCSV", "loop concluído; linhas válidas: %d, erros de parsing: %d", len(rows), parseErrs)
	return rows, parseErrs, nil
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
	vlog.Printf("estoque.go", "UpsertAll", "declarando constante query")
	const query = `
		INSERT INTO estoque
			(data_snapshot, sku, saldo, ruptura)
		VALUES
			(?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			saldo = VALUES(saldo),
			ruptura = VALUES(ruptura)
	`

	vlog.Printf("estoque.go", "UpsertAll", "declarando stmt, err com resultado de db.Prepare()")
	stmt, err := db.Prepare(query)
	vlog.Printf("estoque.go", "UpsertAll", "verificando se err != nil")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("prepare falhou: %w", err)
	}
	vlog.Printf("estoque.go", "UpsertAll", "agendando defer de stmt.Close()")
	defer stmt.Close()

	vlog.Printf("estoque.go", "UpsertAll", "iniciando loop range sobre rows")
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
	vlog.Printf("estoque.go", "UpsertAll", "loop concluído; itens: %d", len(rows))
	return inserted, updated, failed, nil
}
