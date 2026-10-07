// Package visitas implementa o importador usado por cmd/importvisitas: lê
// dados/crm/visitas.csv e importa (upsert) os registros na tabela `visitas`
// (ver sql/16_ddl_visitas.sql).
//
// Uso (via comando):
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
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
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
//     atualizadas (update) e com erro — e devolve erro (o comando encerra com
//     log.Fatalf) se houver erro de conexão ou de leitura do arquivo.
package visitas

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
	"github.com/rotaperfumes/shared/vlog"
)

// Tag prefixa os logs e as mensagens de erro do importador.
const Tag = "importvisitas"

// EnvCSVPath sobrescreve o caminho default do CSV quando a flag -csv não é informada.
const EnvCSVPath = "VISITAS_CSV_PATH"

// dateLayouts são os formatos de data aceitos no CSV (mesmo padrão de importoportunidades).
var dateLayouts = []string{"2006-01-02", "02/01/2006"}

// Row é uma linha já normalizada do CSV, pronta para o upsert.
type Row struct {
	VisitaID        int64
	ClienteIDOrigem int64
	VendedorID      int64
	DataVisita      time.Time
	Resultado       string
	DuracaoMin      int64
}

// Options configura uma execução do importador.
type Options struct {
	// CSVFlag é o valor da flag -csv (vazio = env VISITAS_CSV_PATH ou default).
	CSVFlag string
	// DryRun faz parsing e validação sem abrir o banco.
	DryRun bool
}

// Run executa a importação completa: resolve e lê o CSV, abre o banco via
// open (só se não for dry-run), aplica a unificação de clientes (NEG-01) e
// faz o upsert. Erros fatais são devolvidos sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	vlog.Printf("visitas.go", "Run", "declarando csvPath, err com resultado de ResolveCSVPath()")
	csvPath, err := ResolveCSVPath(opts.CSVFlag)
	vlog.Printf("visitas.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	log.Printf("importvisitas: lendo CSV de %s", csvPath)

	vlog.Printf("visitas.go", "Run", "declarando rows, parseErrs, err com resultado de ReadCSVFile()")
	rows, parseErrs, err := ReadCSVFile(csvPath)
	vlog.Printf("visitas.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("falha ao ler CSV: %w", err)
	}
	log.Printf("importvisitas: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	vlog.Printf("visitas.go", "Run", "verificando se opts.DryRun")
	if opts.DryRun {
		log.Printf("importvisitas: --dry-run informado, nada foi gravado no banco")
		return nil
	}

	vlog.Printf("visitas.go", "Run", "declarando db, err com resultado de open()")
	db, err := open()
	vlog.Printf("visitas.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	vlog.Printf("visitas.go", "Run", "agendando defer de db.Close()")
	defer db.Close()

	vlog.Printf("visitas.go", "Run", "chamando db.Ping() e verificando se err != nil")
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping no banco falhou: %w", err)
	}

	vlog.Printf("visitas.go", "Run", "declarando clienteIDs, err com resultado de LoadClienteIDsByOrigem()")
	clienteIDs, err := LoadClienteIDsByOrigem(db)
	vlog.Printf("visitas.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("falha ao carregar lookup de clientes: %w", err)
	}
	log.Printf("importvisitas: %d clientes carregados para lookup", len(clienteIDs))
	// NEG-01: cópias de CNPJ duplicado no clientes.csv apontam para o sobrevivente.
	vlog.Printf("visitas.go", "Run", "declarando projectRoot, _ com resultado de cmdutil.FindProjectRoot()")
	projectRoot, _ := cmdutil.FindProjectRoot()
	vlog.Printf("visitas.go", "Run", "chamando clientesdedup.Redirecionar()")
	clientesdedup.Redirecionar(Tag, projectRoot, clienteIDs)

	vlog.Printf("visitas.go", "Run", "declarando inserted, updated, failed, err com resultado de UpsertAll()")
	inserted, updated, failed, err := UpsertAll(db, rows, clienteIDs)
	vlog.Printf("visitas.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}

	log.Printf("importvisitas: OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(rows), inserted, updated, failed, parseErrs)
	return nil
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env VISITAS_CSV_PATH > default (dados/crm/visitas.csv na raiz do projeto).
func ResolveCSVPath(flagValue string) (string, error) {
	vlog.Printf("visitas.go", "ResolveCSVPath", "verificando condição do if")
	if flagValue != "" {
		return flagValue, nil
	}
	vlog.Printf("visitas.go", "ResolveCSVPath", "chamando os.Getenv() e verificando condição do if")
	if v := os.Getenv(EnvCSVPath); v != "" {
		return v, nil
	}
	vlog.Printf("visitas.go", "ResolveCSVPath", "declarando root, err com resultado de cmdutil.FindProjectRoot()")
	root, err := cmdutil.FindProjectRoot()
	vlog.Printf("visitas.go", "ResolveCSVPath", "verificando se err != nil")
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a raiz do projeto: %w", err)
	}
	return filepath.Join(root, "dados", "crm", "visitas.csv"), nil
}

// ReadCSVFile abre o arquivo em path e delega para ReadCSV.
func ReadCSVFile(path string) (rows []Row, parseErrs int, err error) {
	vlog.Printf("visitas.go", "ReadCSVFile", "declarando f, err com resultado de os.Open()")
	f, err := os.Open(path)
	vlog.Printf("visitas.go", "ReadCSVFile", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	vlog.Printf("visitas.go", "ReadCSVFile", "agendando defer de f.Close()")
	defer f.Close()
	return ReadCSV(f)
}

// ReadCSV lê e normaliza o CSV (com cabeçalho). Linhas malformadas são
// contadas em parseErrs e puladas (não abortam a importação inteira).
func ReadCSV(rd io.Reader) (rows []Row, parseErrs int, err error) {
	vlog.Printf("visitas.go", "ReadCSV", "declarando r com resultado de csv.NewReader()")
	r := csv.NewReader(rd)
	vlog.Printf("visitas.go", "ReadCSV", "atribuindo a r.FieldsPerRecord o valor de valor literal")
	r.FieldsPerRecord = 6

	// Descarta o cabeçalho.
	vlog.Printf("visitas.go", "ReadCSV", "chamando r.Read() e verificando se err != nil")
	if _, err := r.Read(); err != nil {
		return nil, 0, fmt.Errorf("lendo cabeçalho: %w", err)
	}

	vlog.Printf("visitas.go", "ReadCSV", "declarando lineNum com valor literal")
	lineNum := 1
	vlog.Printf("visitas.go", "ReadCSV", "iniciando loop for sem condição (até break)")
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

		row, err := ParseRow(record)
		if err != nil {
			log.Printf("importvisitas: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	vlog.Printf("visitas.go", "ReadCSV", "loop concluído; linhas válidas: %d, erros de parsing: %d", len(rows), parseErrs)
	return rows, parseErrs, nil
}

// ParseRow converte um registro CSV bruto em Row, aplicando TRIM e
// parsing de data (múltiplos formatos).
func ParseRow(record []string) (Row, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	visitaID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("visita_id inválido (%q): %w", record[0], err)
	}

	clienteID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("cliente_id inválido (%q): %w", record[1], err)
	}

	vendedorID, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("vendedor_id inválido (%q): %w", record[2], err)
	}

	dataVisita, err := ParseData(record[3])
	if err != nil {
		return Row{}, fmt.Errorf("data_visita inválida (%q): %w", record[3], err)
	}

	resultado := record[4]
	if resultado == "" {
		return Row{}, fmt.Errorf("resultado vazio")
	}

	duracaoMin, err := strconv.ParseInt(record[5], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("duracao_min inválido (%q): %w", record[5], err)
	}

	return Row{
		VisitaID:        visitaID,
		ClienteIDOrigem: clienteID,
		VendedorID:      vendedorID,
		DataVisita:      dataVisita,
		Resultado:       resultado,
		DuracaoMin:      duracaoMin,
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
// usado como FK em visitas.cliente_id é o próprio cliente_id_origem — o mapa
// serve apenas para checar existência (identidade origem -> origem).
func LoadClienteIDsByOrigem(db cmdutil.DB) (map[int64]int64, error) {
	vlog.Printf("visitas.go", "LoadClienteIDsByOrigem", "declarando rows, err com resultado de db.Query()")
	rows, err := db.Query("SELECT cliente_id_origem FROM clientes")
	vlog.Printf("visitas.go", "LoadClienteIDsByOrigem", "verificando se err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("visitas.go", "LoadClienteIDsByOrigem", "agendando defer de rows.Close()")
	defer rows.Close()

	vlog.Printf("visitas.go", "LoadClienteIDsByOrigem", "declarando m com resultado de make()")
	m := make(map[int64]int64)
	vlog.Printf("visitas.go", "LoadClienteIDsByOrigem", "iniciando loop for enquanto rows.Next()")
	for rows.Next() {
		var origem int64
		if err := rows.Scan(&origem); err != nil {
			return nil, err
		}
		m[origem] = origem
	}
	vlog.Printf("visitas.go", "LoadClienteIDsByOrigem", "loop concluído; registros carregados: %d", len(m))
	return m, rows.Err()
}

// UpsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY
// UPDATE, usando o próprio `visita_id` (PK AUTO_INCREMENT, vindo
// explicitamente do CSV) como chave de idempotência — mesmo padrão de
// importoportunidades. Linhas cujo cliente_id não é encontrado no lookup são
// contadas como erro e puladas; vendedor_id não é validado em memória (FK do
// banco garante a integridade e retorna erro no upsert, mesmo padrão de
// importoportunidades). Só devolve err se o prepare falhar.
func UpsertAll(db cmdutil.DB, rows []Row, clienteIDs map[int64]int64) (inserted, updated, failed int, err error) {
	vlog.Printf("visitas.go", "UpsertAll", "declarando constante query")
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

	vlog.Printf("visitas.go", "UpsertAll", "declarando stmt, err com resultado de db.Prepare()")
	stmt, err := db.Prepare(query)
	vlog.Printf("visitas.go", "UpsertAll", "verificando se err != nil")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("prepare falhou: %w", err)
	}
	vlog.Printf("visitas.go", "UpsertAll", "agendando defer de stmt.Close()")
	defer stmt.Close()

	vlog.Printf("visitas.go", "UpsertAll", "iniciando loop range sobre rows")
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
	vlog.Printf("visitas.go", "UpsertAll", "loop concluído; itens: %d", len(rows))
	return inserted, updated, failed, nil
}
