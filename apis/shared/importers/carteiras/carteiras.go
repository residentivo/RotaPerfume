// Package carteiras implementa o importador usado por cmd/importcarteiras:
// lê dados/crm/carteira.csv e importa (upsert) os registros na tabela
// `carteiras` (ver sql/14_ddl_carteiras.sql).
//
// Uso (via comando):
//
//	cd apis/shared && go run ./cmd/importcarteiras
//	cd apis/shared && go run ./cmd/importcarteiras -csv=/caminho/alternativo/carteira.csv
//	cd apis/shared && go run ./cmd/importcarteiras -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-carteiras
//
// Pré-requisito: `clientes` e `vendedores` já devem estar importados/seedados
// (o lookup de cliente_id depende disso; vendedor_id do CSV já corresponde
// 1:1 ao id de vendedores, sem necessidade de lookup).
//
// Comportamento:
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
//  2. Lê o CSV informado via flag -csv (default: dados/crm/carteira.csv, resolvido
//     a partir da raiz do repositório) ou via env CARTEIRAS_CSV_PATH.
//  3. Faz parsing linha a linha com TRIM em todos os campos:
//     carteira_id,cliente_id,vendedor_id,data_inicio,data_fim
//     - data_inicio: aceita "2006-01-02" e "02/01/2006".
//     - data_fim: mesmos formatos, mas pode vir vazio (vínculo ainda ativo).
//  4. Resolve `cliente_id` do CSV para o `id` interno de `clientes` via lookup
//     em `clientes.cliente_id_origem` (pré-carregado em memória).
//  5. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando
//     `carteira_id_origem` como chave de idempotência (UNIQUE KEY na tabela).
//  6. Loga contadores finais: total de linhas lidas, importadas (insert),
//     atualizadas (update), ignoradas e com erro — e devolve erro (o comando
//     encerra com log.Fatalf) se houver erro de conexão ou de leitura do arquivo.
package carteiras

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
const Tag = "importcarteiras"

// EnvCSVPath sobrescreve o caminho default do CSV quando a flag -csv não é informada.
const EnvCSVPath = "CARTEIRAS_CSV_PATH"

// dateLayouts são os formatos de data aceitos no CSV (mesmo padrão de importclientes).
var dateLayouts = []string{"2006-01-02", "02/01/2006"}

// Row é uma linha já normalizada do CSV, pronta para o upsert.
type Row struct {
	CarteiraIDOrigem int64
	ClienteIDOrigem  int64
	VendedorID       int64
	DataInicio       time.Time
	DataFim          *time.Time
}

// Options configura uma execução do importador.
type Options struct {
	// CSVFlag é o valor da flag -csv (vazio = env CARTEIRAS_CSV_PATH ou default).
	CSVFlag string
	// DryRun faz parsing e validação sem abrir o banco.
	DryRun bool
}

// Run executa a importação completa: resolve e lê o CSV, abre o banco via
// open (só se não for dry-run), aplica a unificação de clientes (NEG-01) e
// faz o upsert. Erros fatais são devolvidos sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	vlog.Printf("carteiras.go", "Run", "declarando csvPath, err com resultado de ResolveCSVPath()")
	csvPath, err := ResolveCSVPath(opts.CSVFlag)
	vlog.Printf("carteiras.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	log.Printf("importcarteiras: lendo CSV de %s", csvPath)

	vlog.Printf("carteiras.go", "Run", "declarando rows, parseErrs, err com resultado de ReadCSVFile()")
	rows, parseErrs, err := ReadCSVFile(csvPath)
	vlog.Printf("carteiras.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("falha ao ler CSV: %w", err)
	}
	log.Printf("importcarteiras: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	vlog.Printf("carteiras.go", "Run", "verificando se opts.DryRun")
	if opts.DryRun {
		log.Printf("importcarteiras: --dry-run informado, nada foi gravado no banco")
		return nil
	}

	vlog.Printf("carteiras.go", "Run", "declarando db, err com resultado de open()")
	db, err := open()
	vlog.Printf("carteiras.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}
	vlog.Printf("carteiras.go", "Run", "agendando defer de db.Close()")
	defer db.Close()

	vlog.Printf("carteiras.go", "Run", "chamando db.Ping() e verificando se err != nil")
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping no banco falhou: %w", err)
	}

	vlog.Printf("carteiras.go", "Run", "declarando clienteIDs, err com resultado de LoadClienteIDsByOrigem()")
	clienteIDs, err := LoadClienteIDsByOrigem(db)
	vlog.Printf("carteiras.go", "Run", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("falha ao carregar lookup de clientes: %w", err)
	}
	log.Printf("importcarteiras: %d clientes carregados para lookup", len(clienteIDs))

	// NEG-01: cópias de CNPJ duplicado no clientes.csv apontam para o
	// sobrevivente; vínculos da cópia equivalentes a um já existente no
	// grupo são descartados (mesma regra de sql/19_alter_clientes_cnpj_unique.sql).
	vlog.Printf("carteiras.go", "Run", "declarando lidos com resultado de len()")
	lidos := len(rows)
	vlog.Printf("carteiras.go", "Run", "declarando projectRoot, _ com resultado de cmdutil.FindProjectRoot()")
	projectRoot, _ := cmdutil.FindProjectRoot()
	vlog.Printf("carteiras.go", "Run", "declarando unificacao com resultado de clientesdedup.Redirecionar()")
	unificacao := clientesdedup.Redirecionar(Tag, projectRoot, clienteIDs)
	vlog.Printf("carteiras.go", "Run", "declarando rows, descartadas com resultado de DescartarVinculosEquivalentes()")
	rows, descartadas := DescartarVinculosEquivalentes(rows, unificacao)
	log.Printf("importcarteiras: %d carteira(s) de cliente unificado descartada(s) por vínculo equivalente (mesmo vendedor e data_inicio)", descartadas)

	vlog.Printf("carteiras.go", "Run", "declarando inserted, updated, failed, err com resultado de UpsertAll()")
	inserted, updated, failed, err := UpsertAll(db, rows, clienteIDs)
	vlog.Printf("carteiras.go", "Run", "verificando se err != nil")
	if err != nil {
		return err
	}

	log.Printf("importcarteiras: OK — lidos=%d descartados_unificacao=%d inseridos_ou_inalterados=%d atualizados=%d erros_upsert=%d erros_parsing=%d",
		lidos, descartadas, inserted, updated, failed, parseErrs)
	return nil
}

// chaveVinculo identifica um vínculo de carteira após a unificação de
// clientes: espelha a UNIQUE (cliente_id, vendedor_id, data_inicio).
type chaveVinculo struct {
	clienteID  int64
	vendedorID int64
	dataInicio string
}

// DescartarVinculosEquivalentes remove, entre as linhas cujo cliente foi
// unificado, as que colidiriam na UNIQUE (cliente_id, vendedor_id,
// data_inicio) com outra linha do mesmo grupo. Sem isso, o ON DUPLICATE KEY
// UPDATE sobrescreveria o data_fim do vínculo do sobrevivente com o da cópia.
// Fica a linha do próprio sobrevivente; se ele não tiver, a de menor
// carteira_id_origem. A ordem original das linhas mantidas é preservada.
func DescartarVinculosEquivalentes(rows []Row, u clientesdedup.Unificacao) (mantidas []Row, descartadas int) {
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "verificando se len(u) == 0")
	if len(u) == 0 {
		return rows, 0
	}
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "declarando chave com função anônima")
	chave := func(r Row) chaveVinculo {
		return chaveVinculo{u.Canonico(r.ClienteIDOrigem), r.VendedorID, r.DataInicio.Format("2006-01-02")}
	}
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "declarando preferida com função anônima")
	preferida := func(a, b Row) bool { // a é preferível a b?
		vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes.func", "declarando aSobrevivente com expressão u.Canonico(a.ClienteIDOrigem) == a.ClienteIDOrigem")
		aSobrevivente := u.Canonico(a.ClienteIDOrigem) == a.ClienteIDOrigem
		vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes.func", "declarando bSobrevivente com expressão u.Canonico(b.ClienteIDOrigem) == b.ClienteIDOrigem")
		bSobrevivente := u.Canonico(b.ClienteIDOrigem) == b.ClienteIDOrigem
		vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes.func", "verificando se aSobrevivente != bSobrevivente")
		if aSobrevivente != bSobrevivente {
			return aSobrevivente
		}
		return a.CarteiraIDOrigem < b.CarteiraIDOrigem
	}

	// Só entram na disputa as linhas de clientes de um grupo unificado
	// (cópia ou sobrevivente); as demais passam intactas.
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "declarando sobreviventes com resultado de make()")
	sobreviventes := make(map[int64]bool, len(u))
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "iniciando loop range sobre u")
	for _, s := range u {
		sobreviventes[s] = true
	}
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "loop concluído; itens: %d", len(u))
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "declarando doGrupo com função anônima")
	doGrupo := func(r Row) bool {
		vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes.func", "declarando _, copia com elemento indexado de u")
		_, copia := u[r.ClienteIDOrigem]
		return copia || sobreviventes[r.ClienteIDOrigem]
	}

	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "declarando vencedora com resultado de make()")
	vencedora := make(map[chaveVinculo]Row, len(rows))
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "iniciando loop range sobre rows")
	for _, r := range rows {
		if !doGrupo(r) {
			continue
		}
		k := chave(r)
		if atual, ok := vencedora[k]; !ok || preferida(r, atual) {
			vencedora[k] = r
		}
	}
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "loop concluído; itens: %d", len(rows))

	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "atribuindo a mantidas o valor de resultado de make()")
	mantidas = make([]Row, 0, len(rows))
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "iniciando loop range sobre rows")
	for _, r := range rows {
		if doGrupo(r) && vencedora[chave(r)].CarteiraIDOrigem != r.CarteiraIDOrigem {
			log.Printf("importcarteiras: carteira_id_origem=%d descartada: vínculo equivalente ao de carteira_id_origem=%d após unificar o cliente_id=%d",
				r.CarteiraIDOrigem, vencedora[chave(r)].CarteiraIDOrigem, r.ClienteIDOrigem)
			descartadas++
			continue
		}
		mantidas = append(mantidas, r)
	}
	vlog.Printf("carteiras.go", "DescartarVinculosEquivalentes", "loop concluído; itens: %d", len(rows))
	return mantidas, descartadas
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env CARTEIRAS_CSV_PATH > default (dados/crm/carteira.csv na raiz do projeto).
func ResolveCSVPath(flagValue string) (string, error) {
	vlog.Printf("carteiras.go", "ResolveCSVPath", "verificando condição do if")
	if flagValue != "" {
		return flagValue, nil
	}
	vlog.Printf("carteiras.go", "ResolveCSVPath", "chamando os.Getenv() e verificando condição do if")
	if v := os.Getenv(EnvCSVPath); v != "" {
		return v, nil
	}
	vlog.Printf("carteiras.go", "ResolveCSVPath", "declarando root, err com resultado de cmdutil.FindProjectRoot()")
	root, err := cmdutil.FindProjectRoot()
	vlog.Printf("carteiras.go", "ResolveCSVPath", "verificando se err != nil")
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a raiz do projeto: %w", err)
	}
	return filepath.Join(root, "dados", "crm", "carteira.csv"), nil
}

// ReadCSVFile abre o arquivo em path e delega para ReadCSV.
func ReadCSVFile(path string) (rows []Row, parseErrs int, err error) {
	vlog.Printf("carteiras.go", "ReadCSVFile", "declarando f, err com resultado de os.Open()")
	f, err := os.Open(path)
	vlog.Printf("carteiras.go", "ReadCSVFile", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	vlog.Printf("carteiras.go", "ReadCSVFile", "agendando defer de f.Close()")
	defer f.Close()
	return ReadCSV(f)
}

// ReadCSV lê e normaliza o CSV (com cabeçalho). Linhas malformadas são
// contadas em parseErrs e puladas (não abortam a importação inteira).
func ReadCSV(rd io.Reader) (rows []Row, parseErrs int, err error) {
	vlog.Printf("carteiras.go", "ReadCSV", "declarando r com resultado de csv.NewReader()")
	r := csv.NewReader(rd)
	vlog.Printf("carteiras.go", "ReadCSV", "atribuindo a r.FieldsPerRecord o valor de valor literal")
	r.FieldsPerRecord = 5

	// Descarta o cabeçalho.
	vlog.Printf("carteiras.go", "ReadCSV", "chamando r.Read() e verificando se err != nil")
	if _, err := r.Read(); err != nil {
		return nil, 0, fmt.Errorf("lendo cabeçalho: %w", err)
	}

	vlog.Printf("carteiras.go", "ReadCSV", "declarando lineNum com valor literal")
	lineNum := 1
	vlog.Printf("carteiras.go", "ReadCSV", "iniciando loop for sem condição (até break)")
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		lineNum++
		if err != nil {
			log.Printf("importcarteiras: linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := ParseRow(record)
		if err != nil {
			log.Printf("importcarteiras: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	vlog.Printf("carteiras.go", "ReadCSV", "loop concluído; linhas válidas: %d, erros de parsing: %d", len(rows), parseErrs)
	return rows, parseErrs, nil
}

// ParseRow converte um registro CSV bruto em Row, aplicando TRIM e
// parsing de datas (múltiplos formatos, data_fim opcional).
func ParseRow(record []string) (Row, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	carteiraID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("carteira_id inválido (%q): %w", record[0], err)
	}

	clienteID, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("cliente_id inválido (%q): %w", record[1], err)
	}

	vendedorID, err := strconv.ParseInt(record[2], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("vendedor_id inválido (%q): %w", record[2], err)
	}

	dataInicio, err := ParseData(record[3])
	if err != nil {
		return Row{}, fmt.Errorf("data_inicio inválida (%q): %w", record[3], err)
	}

	var dataFim *time.Time
	if record[4] != "" {
		df, err := ParseData(record[4])
		if err != nil {
			return Row{}, fmt.Errorf("data_fim inválida (%q): %w", record[4], err)
		}
		dataFim = &df
	}

	return Row{
		CarteiraIDOrigem: carteiraID,
		ClienteIDOrigem:  clienteID,
		VendedorID:       vendedorID,
		DataInicio:       dataInicio,
		DataFim:          dataFim,
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
// cliente_id_origem agora É a PK da tabela (ver sql/09_ddl_clientes.sql), o
// valor usado como FK em carteiras.cliente_id é o próprio cliente_id_origem
// — o mapa serve apenas para checar existência (identidade origem -> origem).
func LoadClienteIDsByOrigem(db cmdutil.DB) (map[int64]int64, error) {
	vlog.Printf("carteiras.go", "LoadClienteIDsByOrigem", "declarando rows, err com resultado de db.Query()")
	rows, err := db.Query("SELECT cliente_id_origem FROM clientes")
	vlog.Printf("carteiras.go", "LoadClienteIDsByOrigem", "verificando se err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("carteiras.go", "LoadClienteIDsByOrigem", "agendando defer de rows.Close()")
	defer rows.Close()

	vlog.Printf("carteiras.go", "LoadClienteIDsByOrigem", "declarando m com resultado de make()")
	m := make(map[int64]int64)
	vlog.Printf("carteiras.go", "LoadClienteIDsByOrigem", "iniciando loop for enquanto rows.Next()")
	for rows.Next() {
		var origem int64
		if err := rows.Scan(&origem); err != nil {
			return nil, err
		}
		m[origem] = origem
	}
	vlog.Printf("carteiras.go", "LoadClienteIDsByOrigem", "loop concluído; registros carregados: %d", len(m))
	return m, rows.Err()
}

// UpsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY UPDATE,
// usando carteira_id_origem como chave de idempotência. Linhas cujo cliente_id
// não é encontrado no lookup são contadas como erro e puladas. Só devolve err
// se o prepare falhar (a importação não pode prosseguir).
func UpsertAll(db cmdutil.DB, rows []Row, clienteIDs map[int64]int64) (inserted, updated, failed int, err error) {
	vlog.Printf("carteiras.go", "UpsertAll", "declarando constante query")
	const query = `
		INSERT INTO carteiras
			(carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim)
		VALUES
			(?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cliente_id = VALUES(cliente_id),
			vendedor_id = VALUES(vendedor_id),
			data_inicio = VALUES(data_inicio),
			data_fim = VALUES(data_fim)
	`

	vlog.Printf("carteiras.go", "UpsertAll", "declarando stmt, err com resultado de db.Prepare()")
	stmt, err := db.Prepare(query)
	vlog.Printf("carteiras.go", "UpsertAll", "verificando se err != nil")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("prepare falhou: %w", err)
	}
	vlog.Printf("carteiras.go", "UpsertAll", "agendando defer de stmt.Close()")
	defer stmt.Close()

	vlog.Printf("carteiras.go", "UpsertAll", "iniciando loop range sobre rows")
	for _, row := range rows {
		clienteID, ok := clienteIDs[row.ClienteIDOrigem]
		if !ok {
			log.Printf("importcarteiras: carteira_id_origem=%d: cliente_id=%d não encontrado em clientes.cliente_id_origem",
				row.CarteiraIDOrigem, row.ClienteIDOrigem)
			failed++
			continue
		}

		var dataFim interface{}
		if row.DataFim != nil {
			dataFim = row.DataFim.Format("2006-01-02")
		}

		result, err := stmt.Exec(
			row.CarteiraIDOrigem, clienteID, row.VendedorID,
			row.DataInicio.Format("2006-01-02"), dataFim,
		)
		if err != nil {
			log.Printf("importcarteiras: erro no upsert de carteira_id_origem=%d: %v", row.CarteiraIDOrigem, err)
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
	vlog.Printf("carteiras.go", "UpsertAll", "loop concluído; itens: %d", len(rows))
	return inserted, updated, failed, nil
}
