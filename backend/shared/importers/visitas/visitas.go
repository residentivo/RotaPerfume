// Package visitas implementa o importador usado por cmd/importvisitas: lê
// dados/crm/visitas.csv e importa (upsert) os registros na tabela `visitas`
// (ver sql/16_ddl_visitas.sql).
//
// Uso (via comando):
//
//	cd backend/shared && go run ./cmd/importvisitas
//	cd backend/shared && go run ./cmd/importvisitas -csv=/caminho/alternativo/visitas.csv
//	cd backend/shared && go run ./cmd/importvisitas -dry-run
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
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/clientesdedup"
	"github.com/rotaperfumes/shared/importers/importkit"
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
	log.Printf("importvisitas: lendo CSV de %s", csvPath)
	if i.rows, i.invalidas, err = ReadCSVFile(csvPath); err != nil {
		return fmt.Errorf("falha ao ler CSV: %w", err)
	}
	log.Printf("importvisitas: %d linhas válidas lidas, %d linhas com erro de parsing", len(i.rows), i.invalidas)
	return nil
}

func (i *importacao) gravar(db cmdutil.DB) error {
	clienteIDs, err := LoadClienteIDsByOrigem(db)
	if err != nil {
		return fmt.Errorf("falha ao carregar lookup de clientes: %w", err)
	}
	log.Printf("importvisitas: %d clientes carregados para lookup", len(clienteIDs))
	// NEG-01: cópias de CNPJ duplicado no clientes.csv apontam para o sobrevivente.
	projectRoot, _ := cmdutil.FindProjectRoot()
	clientesdedup.Redirecionar(Tag, projectRoot, clienteIDs)

	inserted, updated, failed, err := UpsertAll(db, i.rows, clienteIDs)
	if err != nil {
		return err
	}
	log.Printf("importvisitas: OK — lidos=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		len(i.rows), inserted, updated, failed, i.invalidas)
	return nil
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env VISITAS_CSV_PATH > default (dados/crm/visitas.csv na raiz do projeto).
func ResolveCSVPath(flagValue string) (string, error) {
	return importkit.ResolverCSV(flagValue, EnvCSVPath, "dados", "crm", "visitas.csv")
}

// ReadCSVFile abre o arquivo em path e delega para ReadCSV.
func ReadCSVFile(path string) (rows []Row, parseErrs int, err error) {
	return importkit.LerArquivo(path, ReadCSV)
}

// ReadCSV lê e normaliza o CSV (com cabeçalho). Linhas malformadas são
// contadas em parseErrs e puladas (não abortam a importação inteira).
func ReadCSV(rd io.Reader) (rows []Row, parseErrs int, err error) {
	return importkit.LerCSV(rd, importkit.Formato{Prefixo: "importvisitas:", Campos: 6}, ParseRow)
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
	return importkit.IDsDeOrigem(db, "SELECT cliente_id_origem FROM clientes")
}

// UpsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY
// UPDATE, usando o próprio `visita_id` (PK AUTO_INCREMENT, vindo
// explicitamente do CSV) como chave de idempotência — mesmo padrão de
// importoportunidades. Linhas cujo cliente_id não é encontrado no lookup são
// contadas como erro e puladas; vendedor_id não é validado em memória (FK do
// banco garante a integridade e retorna erro no upsert, mesmo padrão de
// importoportunidades). Só devolve err se o prepare falhar.
func UpsertAll(db cmdutil.DB, rows []Row, clienteIDs map[int64]int64) (inserted, updated, failed int, err error) {
	c, err := importkit.Upsert[Row]{
		SQL: `
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
	`,
		ErroPrepare: "prepare falhou",
		Args: func(row Row) ([]any, bool) {
			clienteID, ok := clienteIDs[row.ClienteIDOrigem]
			if !ok {
				log.Printf("importvisitas: visita_id=%d: cliente_id=%d não encontrado em clientes.cliente_id_origem",
					row.VisitaID, row.ClienteIDOrigem)
				return nil, false
			}
			return []any{row.VisitaID, clienteID, row.VendedorID,
				row.DataVisita.Format("2006-01-02"), row.Resultado, row.DuracaoMin}, true
		},
		LogFalha: func(row Row, err error) {
			log.Printf("importvisitas: erro no upsert de visita_id=%d (vendedor_id=%d): %v",
				row.VisitaID, row.VendedorID, err)
		},
	}.Gravar(db, rows)
	return c.Inseridos, c.Atualizados, c.Falhas, err
}
