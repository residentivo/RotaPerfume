// Package oportunidades implementa o importador usado por
// cmd/importoportunidades: lê dados/crm/oportunidades.csv e importa (upsert)
// os registros na tabela `oportunidades` (ver sql/15_ddl_oportunidades.sql).
//
// Uso (via comando):
//
//	cd apis/shared && go run ./cmd/importoportunidades
//	cd apis/shared && go run ./cmd/importoportunidades -csv=/caminho/alternativo/oportunidades.csv
//	cd apis/shared && go run ./cmd/importoportunidades -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-oportunidades
//
// Pré-requisito: `clientes` e `vendedores` já devem estar importados/seedados
// (o lookup de cliente_id depende disso; vendedor_id do CSV já corresponde
// 1:1 ao id de vendedores, sem necessidade de lookup — mesmo padrão de
// importcarteiras).
//
// Comportamento:
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
//  2. Lê o CSV informado via flag -csv (default: dados/crm/oportunidades.csv,
//     resolvido a partir da raiz do repositório) ou via env
//     OPORTUNIDADES_CSV_PATH.
//  3. Faz parsing linha a linha com TRIM em todos os campos:
//     oportunidade_id,cliente_id,vendedor_id,origem,data_abertura,etapa,
//     probabilidade_pct,valor_estimado,data_fechamento,ciclo_dias,motivo_perda
//     - data_abertura: aceita "2006-01-02" e "02/01/2006".
//     - data_fechamento, ciclo_dias e motivo_perda podem vir vazios (NULL) —
//     oportunidade ainda aberta.
//  4. Resolve `cliente_id` do CSV para o `cliente_id_origem` de `clientes` via
//     lookup pré-carregado em memória (mesmo padrão de importcarteiras).
//  5. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando o próprio
//     `oportunidade_id` (PK AUTO_INCREMENT, valor vindo explicitamente do
//     CSV) como chave de idempotência — mesmo padrão de importpagamentos.
//  6. Loga contadores finais: total de linhas lidas, importadas (insert),
//     atualizadas (update) e com erro — e devolve erro (o comando encerra com
//     log.Fatalf) se houver erro de conexão ou de leitura do arquivo.
package oportunidades

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
	"github.com/rotaperfumes/shared/importers/clientesdedup"
)

// Tag prefixa os logs e as mensagens de erro do importador.
const Tag = "importoportunidades"

// EnvCSVPath sobrescreve o caminho default do CSV quando a flag -csv não é informada.
const EnvCSVPath = "OPORTUNIDADES_CSV_PATH"

// dateLayouts são os formatos de data aceitos no CSV (mesmo padrão de importcarteiras/importclientes).
var dateLayouts = []string{"2006-01-02", "02/01/2006"}

// Row é uma linha já normalizada do CSV, pronta para o upsert.
type Row struct {
	OportunidadeID   int64
	ClienteIDOrigem  int64
	VendedorID       int64
	Origem           string
	DataAbertura     time.Time
	Etapa            string
	ProbabilidadePct float64
	ValorEstimado    float64
	DataFechamento   *time.Time
	CicloDias        sql.NullInt64
	MotivoPerda      sql.NullString
}

// Options configura uma execução do importador.
type Options struct {
	// CSVFlag é o valor da flag -csv (vazio = env OPORTUNIDADES_CSV_PATH ou default).
	CSVFlag string
	// DryRun faz parsing e validação sem abrir o banco.
	DryRun bool
}

// Run executa a importação completa: resolve e lê o CSV, abre o banco via
// open (só se não for dry-run), aplica a unificação de clientes (NEG-01) e
// faz o upsert. Erros fatais são devolvidos sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	csvPath, err := ResolveCSVPath(opts.CSVFlag)
	if err != nil {
		return err
	}
	log.Printf("importoportunidades: lendo CSV de %s", csvPath)

	rows, parseErrs, err := ReadCSVFile(csvPath)
	if err != nil {
		return fmt.Errorf("falha ao ler CSV: %w", err)
	}
	log.Printf("importoportunidades: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	if opts.DryRun {
		log.Printf("importoportunidades: --dry-run informado, nada foi gravado no banco")
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
		return fmt.Errorf("falha ao carregar lookup de clientes: %w", err)
	}
	log.Printf("importoportunidades: %d clientes carregados para lookup", len(clienteIDs))
	// NEG-01: cópias de CNPJ duplicado no clientes.csv apontam para o sobrevivente.
	projectRoot, _ := cmdutil.FindProjectRoot()
	clientesdedup.Redirecionar(Tag, projectRoot, clienteIDs)

	inserted, updated, failed, err := UpsertAll(db, rows, clienteIDs)
	if err != nil {
		return err
	}

	log.Printf("importoportunidades: OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
	return nil
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env OPORTUNIDADES_CSV_PATH > default (dados/crm/oportunidades.csv na raiz do projeto).
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
	return filepath.Join(root, "dados", "crm", "oportunidades.csv"), nil
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
	r.FieldsPerRecord = 11

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
			log.Printf("importoportunidades: linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := ParseRow(record)
		if err != nil {
			log.Printf("importoportunidades: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// ParseRow converte um registro CSV bruto em Row, aplicando TRIM,
// parsing de datas (múltiplos formatos) e tratamento de campos opcionais
// (data_fechamento, ciclo_dias, motivo_perda podem vir vazios).
func ParseRow(record []string) (Row, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	oportunidadeID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("oportunidade_id inválido (%q): %w", record[0], err)
	}

	clienteID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("cliente_id inválido (%q): %w", record[1], err)
	}

	vendedorID, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("vendedor_id inválido (%q): %w", record[2], err)
	}

	origem := record[3]
	if origem == "" {
		return Row{}, fmt.Errorf("origem vazia")
	}

	dataAbertura, err := ParseData(record[4])
	if err != nil {
		return Row{}, fmt.Errorf("data_abertura inválida (%q): %w", record[4], err)
	}

	etapa := record[5]
	if etapa == "" {
		return Row{}, fmt.Errorf("etapa vazia")
	}

	probabilidadePct, err := strconv.ParseFloat(record[6], 64)
	if err != nil {
		return Row{}, fmt.Errorf("probabilidade_pct inválido (%q): %w", record[6], err)
	}

	valorEstimado, err := strconv.ParseFloat(record[7], 64)
	if err != nil {
		return Row{}, fmt.Errorf("valor_estimado inválido (%q): %w", record[7], err)
	}

	var dataFechamento *time.Time
	if record[8] != "" {
		df, err := ParseData(record[8])
		if err != nil {
			return Row{}, fmt.Errorf("data_fechamento inválida (%q): %w", record[8], err)
		}
		dataFechamento = &df
	}

	var cicloDias sql.NullInt64
	if record[9] != "" {
		c, err := strconv.ParseInt(record[9], 10, 64)
		if err != nil {
			return Row{}, fmt.Errorf("ciclo_dias inválido (%q): %w", record[9], err)
		}
		cicloDias = sql.NullInt64{Int64: c, Valid: true}
	}

	var motivoPerda sql.NullString
	if record[10] != "" {
		motivoPerda = sql.NullString{String: record[10], Valid: true}
	}

	return Row{
		OportunidadeID:   oportunidadeID,
		ClienteIDOrigem:  clienteID,
		VendedorID:       vendedorID,
		Origem:           origem,
		DataAbertura:     dataAbertura,
		Etapa:            etapa,
		ProbabilidadePct: probabilidadePct,
		ValorEstimado:    valorEstimado,
		DataFechamento:   dataFechamento,
		CicloDias:        cicloDias,
		MotivoPerda:      motivoPerda,
	}, nil
}

// ParseData tenta os layouts conhecidos de data observados no CSV real.
func ParseData(raw string) (time.Time, error) {
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

// LoadClienteIDsByOrigem pré-carrega o conjunto de cliente_id_origem
// existentes em `clientes`, usado para validar o cliente_id do CSV. Como
// cliente_id_origem É a PK da tabela (ver sql/09_ddl_clientes.sql), o valor
// usado como FK em oportunidades.cliente_id é o próprio cliente_id_origem —
// o mapa serve apenas para checar existência (identidade origem -> origem).
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

// UpsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY
// UPDATE, usando o próprio `oportunidade_id` (PK AUTO_INCREMENT, vindo
// explicitamente do CSV) como chave de idempotência — mesmo padrão de
// importpagamentos. Linhas cujo cliente_id não é encontrado no lookup são
// contadas como erro e puladas; vendedor_id não é validado em memória (FK do
// banco garante a integridade e retorna erro no upsert, mesmo padrão de
// importcarteiras). Só devolve err se o prepare falhar.
func UpsertAll(db cmdutil.DB, rows []Row, clienteIDs map[int64]int64) (inserted, updated, failed int, err error) {
	const query = `
		INSERT INTO oportunidades
			(oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cliente_id = VALUES(cliente_id),
			vendedor_id = VALUES(vendedor_id),
			origem = VALUES(origem),
			data_abertura = VALUES(data_abertura),
			etapa = VALUES(etapa),
			probabilidade_pct = VALUES(probabilidade_pct),
			valor_estimado = VALUES(valor_estimado),
			data_fechamento = VALUES(data_fechamento),
			ciclo_dias = VALUES(ciclo_dias),
			motivo_perda = VALUES(motivo_perda)
	`

	stmt, err := db.Prepare(query)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("prepare falhou: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		clienteID, ok := clienteIDs[row.ClienteIDOrigem]
		if !ok {
			log.Printf("importoportunidades: oportunidade_id=%d: cliente_id=%d não encontrado em clientes.cliente_id_origem",
				row.OportunidadeID, row.ClienteIDOrigem)
			failed++
			continue
		}

		var dataFechamento interface{}
		if row.DataFechamento != nil {
			dataFechamento = row.DataFechamento.Format("2006-01-02")
		}

		var cicloDias interface{}
		if row.CicloDias.Valid {
			cicloDias = row.CicloDias.Int64
		}

		var motivoPerda interface{}
		if row.MotivoPerda.Valid {
			motivoPerda = row.MotivoPerda.String
		}

		result, err := stmt.Exec(
			row.OportunidadeID, clienteID, row.VendedorID, row.Origem,
			row.DataAbertura.Format("2006-01-02"), row.Etapa, row.ProbabilidadePct,
			row.ValorEstimado, dataFechamento, cicloDias, motivoPerda,
		)
		if err != nil {
			log.Printf("importoportunidades: erro no upsert de oportunidade_id=%d (vendedor_id=%d): %v",
				row.OportunidadeID, row.VendedorID, err)
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
