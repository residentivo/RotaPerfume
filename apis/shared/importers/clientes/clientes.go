// Package clientes implementa o importador usado por cmd/importclientes: lê
// dados/crm/clientes.csv e importa (upsert) os registros na tabela
// `clientes` (ver sql/09_ddl_clientes.sql).
//
// Uso (via comando):
//
//	cd apis/shared && go run ./cmd/importclientes
//	cd apis/shared && go run ./cmd/importclientes -csv=/caminho/alternativo/clientes.csv
//	cd apis/shared && go run ./cmd/importclientes -dry-run
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-import-clientes
//
// Comportamento:
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
//  2. Lê o CSV informado via flag -csv (default: dados/crm/clientes.csv, resolvido
//     a partir da raiz do repositório) ou via env CLIENTES_CSV_PATH.
//  3. Faz parsing linha a linha com TRIM em todos os campos:
//     cliente_id,cnpj,razao_social,segmento,cidade,uf,bairro,data_cadastro,ativo
//     - cnpj: normalização central (clientesdedup.NormalizarCNPJ → shared/cnpj,
//     NEG-02): remove a máscara (". / -" e espaços — o CSV mistura formatos
//     com e sem máscara), converte para MAIÚSCULAS e exige 14 caracteres
//     (12 em [0-9A-Z] + 2 DVs numéricos). Aceita CNPJ numérico e
//     alfanumérico; outro caractere invalida a linha.
//     - data_cadastro: aceita "2006-01-02" e "02/01/2006" (ambos aparecem no CSV real).
//     - ativo: 'S' -> true, 'N' -> false (case-insensitive).
//     3.1 NEG-01: clientes.cnpj é UNIQUE (uq_clientes_cnpj). Linhas com CNPJ já
//     visto antes no CSV são unificadas na 1ª ocorrência (não são gravadas;
//     os importadores dependentes redirecionam o cliente_id delas via
//     importers/clientesdedup). O total unificado vai para o log. O dígito
//     verificador NÃO é validado (dados fictícios; o DV vale só na API).
//     Linha cujo CNPJ já pertence a outro cliente no banco é pulada
//     (conflitos_cnpj) — nunca aborta com 1062.
//  4. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando
//     `cliente_id_origem` como chave de idempotência (UNIQUE KEY na tabela).
//  5. Loga contadores finais: total de linhas lidas, importadas (insert),
//     atualizadas (update), ignoradas e com erro — e devolve erro (o comando
//     encerra com log.Fatalf) se houver erro de conexão ou de leitura do arquivo.
package clientes

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/clientesdedup"
)

// Tag prefixa os logs e as mensagens de erro do importador.
const Tag = "importclientes"

// EnvCSVPath sobrescreve o caminho default do CSV quando a flag -csv não é informada.
const EnvCSVPath = clientesdedup.EnvCSVPath

// dateLayouts são os formatos de data aceitos no CSV (ambos observados nos dados reais).
var dateLayouts = []string{"2006-01-02", "02/01/2006"}

// Row é uma linha já normalizada do CSV, pronta para o upsert.
type Row struct {
	ClienteIDOrigem int64
	CNPJ            string
	RazaoSocial     string
	Segmento        string
	Cidade          string
	UF              string
	Bairro          string
	DataCadastro    time.Time
	Ativo           bool
}

// Options configura uma execução do importador.
type Options struct {
	// CSVFlag é o valor da flag -csv (vazio = env CLIENTES_CSV_PATH ou default).
	CSVFlag string
	// DryRun faz parsing, validação e unificação sem abrir o banco.
	DryRun bool
}

// Run executa a importação completa: resolve e lê o CSV, unifica CNPJs
// duplicados (NEG-01), abre o banco via open (só se não for dry-run) e faz o
// upsert. Erros fatais são devolvidos sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	csvPath, err := ResolveCSVPath(opts.CSVFlag)
	if err != nil {
		return err
	}
	log.Printf("importclientes: lendo CSV de %s", csvPath)

	rows, parseErrs, err := ReadCSVFile(csvPath)
	if err != nil {
		return fmt.Errorf("falha ao ler CSV: %w", err)
	}
	log.Printf("importclientes: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	lidas := len(rows)
	rows, unificadas := UnificarPorCNPJ(rows)
	log.Printf("importclientes: %d linhas unificadas por CNPJ duplicado (mantida a 1ª ocorrência de cada CNPJ); %d clientes a gravar",
		unificadas, len(rows))

	if opts.DryRun {
		log.Printf("importclientes: --dry-run informado, nada foi gravado no banco")
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

	res, err := UpsertAll(db, rows)
	if err != nil {
		return err
	}

	log.Printf("importclientes: OK — lidos=%d unificados=%d inseridos_ou_inalterados=%d atualizados=%d conflitos_cnpj=%d erros_upsert=%d erros_parsing=%d",
		lidas, unificadas, res.Inserted, res.Updated, res.ConflitosCNPJ, res.Failed, parseErrs)
	return nil
}

// UnificarPorCNPJ remove do lote as linhas cujo CNPJ já apareceu antes no
// CSV (NEG-01: clientes.cnpj é UNIQUE). Fica a 1ª ocorrência; as seguintes
// são descartadas e logadas. Os importadores dependentes (carteiras,
// pedidos, oportunidades, visitas) redirecionam o cliente_id das cópias para
// o sobrevivente via clientesdedup. O DV do CNPJ não é validado aqui.
func UnificarPorCNPJ(rows []Row) (mantidas []Row, unificadas int) {
	regs := make([]clientesdedup.Registro, len(rows))
	for i, r := range rows {
		regs[i] = clientesdedup.Registro{ClienteID: r.ClienteIDOrigem, CNPJ: r.CNPJ}
	}
	copias := clientesdedup.Calcular(regs)

	mantidas = make([]Row, 0, len(rows))
	for _, r := range rows {
		if sobrevivente, ehCopia := copias[r.ClienteIDOrigem]; ehCopia {
			log.Printf("importclientes: cliente_id=%d unificado em cliente_id=%d (CNPJ duplicado)", r.ClienteIDOrigem, sobrevivente)
			unificadas++
			continue
		}
		mantidas = append(mantidas, r)
	}
	return mantidas, unificadas
}

// IsDuplicateCNPJ reporta se err é o MySQL 1062 no índice uq_clientes_cnpj
// (o CNPJ do CSV já pertence a outro cliente no banco, ex.: criado pela API).
func IsDuplicateCNPJ(err error) bool {
	var myErr *mysql.MySQLError
	return errors.As(err, &myErr) && myErr.Number == 1062 && strings.Contains(myErr.Message, "uq_clientes_cnpj")
}

// ResolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env CLIENTES_CSV_PATH > default (dados/crm/clientes.csv na raiz do projeto).
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
	return filepath.Join(root, "dados", "crm", "clientes.csv"), nil
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
	r.FieldsPerRecord = 9

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
			log.Printf("importclientes: linha %d: erro de leitura CSV: %v", lineNum, err)
			parseErrs++
			continue
		}

		row, err := ParseRow(record)
		if err != nil {
			log.Printf("importclientes: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// ParseRow converte um registro CSV bruto em Row, aplicando TRIM,
// normalização de CNPJ, parsing de data (múltiplos formatos) e conversão
// de 'S'/'N' para bool.
func ParseRow(record []string) (Row, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	clienteID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return Row{}, fmt.Errorf("cliente_id inválido (%q): %w", record[0], err)
	}

	cnpj, ok := clientesdedup.NormalizarCNPJ(record[1])
	if !ok {
		return Row{}, fmt.Errorf("cnpj inválido (%q, esperado 14 caracteres: 12 em [0-9A-Z] + 2 DVs numéricos, máscara opcional)", record[1])
	}

	razaoSocial := record[2]
	segmento := record[3]
	cidade := record[4]
	uf := strings.ToUpper(record[5])
	bairro := record[6]

	dataCadastro, err := ParseData(record[7])
	if err != nil {
		return Row{}, fmt.Errorf("data_cadastro inválida (%q): %w", record[7], err)
	}

	ativo, err := ParseAtivo(record[8])
	if err != nil {
		return Row{}, fmt.Errorf("ativo inválido (%q): %w", record[8], err)
	}

	if razaoSocial == "" || uf == "" {
		return Row{}, fmt.Errorf("razao_social ou uf vazios")
	}

	return Row{
		ClienteIDOrigem: clienteID,
		CNPJ:            cnpj,
		RazaoSocial:     razaoSocial,
		Segmento:        segmento,
		Cidade:          cidade,
		UF:              uf,
		Bairro:          bairro,
		DataCadastro:    dataCadastro,
		Ativo:           ativo,
	}, nil
}

// ParseData tenta os layouts conhecidos de data_cadastro observados no CSV real.
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

// ParseAtivo converte 'S'/'N' (case-insensitive) para bool.
func ParseAtivo(raw string) (bool, error) {
	switch strings.ToUpper(raw) {
	case "S":
		return true, nil
	case "N":
		return false, nil
	default:
		return false, fmt.Errorf("esperado 'S' ou 'N'")
	}
}

// ResultadoUpsert agrega os contadores do UpsertAll.
type ResultadoUpsert struct {
	Inserted      int // INSERT novo ou linha já existente sem alteração
	Updated       int // UPDATE com alteração real
	ConflitosCNPJ int // CNPJ já pertence a OUTRO cliente no banco (pulado)
	Failed        int // demais erros de banco
}

// UpsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY UPDATE,
// usando cliente_id_origem como chave de idempotência. Só devolve err se não
// for possível carregar os CNPJs existentes ou preparar o statement.
//
// NEG-01: com o índice UNIQUE uq_clientes_cnpj, uma linha cujo CNPJ já
// pertence a OUTRO cliente no banco (ex.: cadastrado pela API) é pulada e
// contada em ConflitosCNPJ. Sem essa checagem prévia, o ON DUPLICATE KEY
// UPDATE sobrescreveria silenciosamente o outro cliente (o conflito seria no
// índice de CNPJ, não na PK) ou falharia com 1062. O 1062 que ainda escapar
// (ex.: escrita concorrente) também é contado como conflito, sem abortar.
func UpsertAll(db cmdutil.DB, rows []Row) (ResultadoUpsert, error) {
	const query = `
		INSERT INTO clientes
			(cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			cnpj = VALUES(cnpj),
			razao_social = VALUES(razao_social),
			segmento = VALUES(segmento),
			cidade = VALUES(cidade),
			uf = VALUES(uf),
			bairro = VALUES(bairro),
			data_cadastro = VALUES(data_cadastro),
			ativo = VALUES(ativo)
	`

	var res ResultadoUpsert

	donos, err := LoadDonosCNPJ(db)
	if err != nil {
		return res, fmt.Errorf("falha ao carregar CNPJs existentes: %w", err)
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		return res, fmt.Errorf("prepare falhou: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		if dono, ok := donos.Dono(row.CNPJ); ok && dono != row.ClienteIDOrigem {
			log.Printf("importclientes: cliente_id_origem=%d pulado: CNPJ já pertence a outro cliente no banco (cliente_id_origem=%d)",
				row.ClienteIDOrigem, dono)
			res.ConflitosCNPJ++
			continue
		}

		result, err := stmt.Exec(
			row.ClienteIDOrigem, row.CNPJ, row.RazaoSocial, row.Segmento,
			row.Cidade, row.UF, row.Bairro, row.DataCadastro.Format("2006-01-02"), row.Ativo,
		)
		if err != nil {
			if IsDuplicateCNPJ(err) {
				log.Printf("importclientes: cliente_id_origem=%d pulado: CNPJ duplicado (1062 uq_clientes_cnpj)", row.ClienteIDOrigem)
				res.ConflitosCNPJ++
				continue
			}
			log.Printf("importclientes: erro no upsert de cliente_id_origem=%d: %v", row.ClienteIDOrigem, err)
			res.Failed++
			continue
		}
		donos.Gravar(row.ClienteIDOrigem, row.CNPJ)

		// Com clientFoundRows=true no DSN (BUG-04), o MySQL devolve, via ON
		// DUPLICATE KEY UPDATE: 1 = INSERT novo OU linha já existente sem
		// alteração; 2 = UPDATE com alteração real. Por isso o contador
		// "inseridos" é rotulado como inseridos_ou_inalterados.
		affected, _ := result.RowsAffected()
		switch affected {
		case 1:
			res.Inserted++
		default:
			// 2 = UPDATE com alteração; 0 não ocorre com clientFoundRows=true
			// (mantido por robustez).
			res.Updated++
		}
	}
	return res, nil
}

// DonosCNPJ mantém, em memória, quem é o dono de cada CNPJ no banco durante
// a importação (cnpj → cliente_id_origem e o inverso, para liberar o CNPJ
// antigo quando uma linha troca de CNPJ). Crie com NovosDonosCNPJ.
type DonosCNPJ struct {
	porCNPJ map[string]int64
	porID   map[int64]string
}

// NovosDonosCNPJ devolve um DonosCNPJ vazio.
func NovosDonosCNPJ() DonosCNPJ {
	return DonosCNPJ{porCNPJ: map[string]int64{}, porID: map[int64]string{}}
}

// Dono devolve o cliente_id_origem dono do CNPJ, se houver.
func (d DonosCNPJ) Dono(cnpj string) (int64, bool) {
	id, ok := d.porCNPJ[cnpj]
	return id, ok
}

// Gravar registra id como dono de cnpj, liberando o CNPJ anterior de id.
func (d DonosCNPJ) Gravar(id int64, cnpj string) {
	if antigo, ok := d.porID[id]; ok && antigo != cnpj {
		delete(d.porCNPJ, antigo)
	}
	d.porID[id] = cnpj
	d.porCNPJ[cnpj] = id
}

// LoadDonosCNPJ carrega cnpj → cliente_id_origem de todos os clientes do banco.
func LoadDonosCNPJ(db cmdutil.DB) (DonosCNPJ, error) {
	d := NovosDonosCNPJ()
	rows, err := db.Query("SELECT cliente_id_origem, cnpj FROM clientes")
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var cnpj string
		if err := rows.Scan(&id, &cnpj); err != nil {
			return d, err
		}
		d.Gravar(id, cnpj)
	}
	return d, rows.Err()
}
