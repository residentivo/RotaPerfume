// Command importclientes lê dados/crm/clientes.csv e importa (upsert) os
// registros na tabela `clientes` (ver sql/09_ddl_clientes.sql).
//
// Uso:
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
//  1. Carrega .env da raiz do projeto (mesmo mecanismo de seedusers/resetpassword).
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
//     cmd/internal/clientesdedup). O total unificado vai para o log. O dígito
//     verificador NÃO é validado (dados fictícios; o DV vale só na API).
//     Linha cujo CNPJ já pertence a outro cliente no banco é pulada
//     (conflitos_cnpj) — nunca aborta com 1062.
//  4. Faz upsert via INSERT ... ON DUPLICATE KEY UPDATE usando
//     `cliente_id_origem` como chave de idempotência (UNIQUE KEY na tabela).
//  5. Loga contadores finais: total de linhas lidas, importadas (insert),
//     atualizadas (update), ignoradas e com erro — e falha (log.Fatalf) se
//     houver erro de conexão ou de leitura do arquivo.
package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"github.com/rotaperfumes/shared/cmd/internal/clientesdedup"
	"github.com/rotaperfumes/shared/config"
)

// dateLayouts são os formatos de data aceitos no CSV (ambos observados nos dados reais).
var dateLayouts = []string{"2006-01-02", "02/01/2006"}

// clienteRow é uma linha já normalizada do CSV, pronta para o upsert.
type clienteRow struct {
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

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de clientes (default: dados/crm/clientes.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("importclientes: falha ao carregar config: %v", err)
	}

	csvPath := resolveCSVPath(*csvPathFlag)
	log.Printf("importclientes: lendo CSV de %s", csvPath)

	rows, parseErrs, err := readCSV(csvPath)
	if err != nil {
		log.Fatalf("importclientes: falha ao ler CSV: %v", err)
	}
	log.Printf("importclientes: %d linhas válidas lidas, %d linhas com erro de parsing", len(rows), parseErrs)

	lidas := len(rows)
	rows, unificadas := unificarPorCNPJ(rows)
	log.Printf("importclientes: %d linhas unificadas por CNPJ duplicado (mantida a 1ª ocorrência de cada CNPJ); %d clientes a gravar",
		unificadas, len(rows))

	if *dryRun {
		log.Printf("importclientes: --dry-run informado, nada foi gravado no banco")
		return
	}

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("importclientes: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("importclientes: ping no banco falhou: %v", err)
	}

	res := upsertAll(db, rows)

	log.Printf("importclientes: OK — lidos=%d unificados=%d inseridos_ou_inalterados=%d atualizados=%d conflitos_cnpj=%d erros_upsert=%d erros_parsing=%d",
		lidas, unificadas, res.inserted, res.updated, res.conflitosCNPJ, res.failed, parseErrs)
}

// unificarPorCNPJ remove do lote as linhas cujo CNPJ já apareceu antes no
// CSV (NEG-01: clientes.cnpj é UNIQUE). Fica a 1ª ocorrência; as seguintes
// são descartadas e logadas. Os importadores dependentes (carteiras,
// pedidos, oportunidades, visitas) redirecionam o cliente_id das cópias para
// o sobrevivente via clientesdedup. O DV do CNPJ não é validado aqui.
func unificarPorCNPJ(rows []clienteRow) (mantidas []clienteRow, unificadas int) {
	regs := make([]clientesdedup.Registro, len(rows))
	for i, r := range rows {
		regs[i] = clientesdedup.Registro{ClienteID: r.ClienteIDOrigem, CNPJ: r.CNPJ}
	}
	copias := clientesdedup.Calcular(regs)

	mantidas = make([]clienteRow, 0, len(rows))
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

// isDuplicateCNPJ reporta se err é o MySQL 1062 no índice uq_clientes_cnpj
// (o CNPJ do CSV já pertence a outro cliente no banco, ex.: criado pela API).
func isDuplicateCNPJ(err error) bool {
	var myErr *mysql.MySQLError
	return errors.As(err, &myErr) && myErr.Number == 1062 && strings.Contains(myErr.Message, "uq_clientes_cnpj")
}

// resolveCSVPath decide o caminho final do CSV, na ordem:
// flag -csv > env CLIENTES_CSV_PATH > default (dados/crm/clientes.csv na raiz do projeto).
func resolveCSVPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("CLIENTES_CSV_PATH"); v != "" {
		return v
	}
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("importclientes: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "dados", "crm", "clientes.csv")
}

// readCSV lê e normaliza o arquivo CSV. Linhas malformadas são contadas em
// parseErrs e puladas (não abortam a importação inteira).
func readCSV(path string) (rows []clienteRow, parseErrs int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("abrindo arquivo: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
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

		row, err := parseRow(record)
		if err != nil {
			log.Printf("importclientes: linha %d: %v", lineNum, err)
			parseErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, parseErrs, nil
}

// parseRow converte um registro CSV bruto em clienteRow, aplicando TRIM,
// normalização de CNPJ, parsing de data (múltiplos formatos) e conversão
// de 'S'/'N' para bool.
func parseRow(record []string) (clienteRow, error) {
	for i := range record {
		record[i] = strings.TrimSpace(record[i])
	}

	clienteID, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return clienteRow{}, fmt.Errorf("cliente_id inválido (%q): %w", record[0], err)
	}

	cnpj, ok := clientesdedup.NormalizarCNPJ(record[1])
	if !ok {
		return clienteRow{}, fmt.Errorf("cnpj inválido (%q, esperado 14 caracteres: 12 em [0-9A-Z] + 2 DVs numéricos, máscara opcional)", record[1])
	}

	razaoSocial := record[2]
	segmento := record[3]
	cidade := record[4]
	uf := strings.ToUpper(record[5])
	bairro := record[6]

	dataCadastro, err := parseData(record[7])
	if err != nil {
		return clienteRow{}, fmt.Errorf("data_cadastro inválida (%q): %w", record[7], err)
	}

	ativo, err := parseAtivo(record[8])
	if err != nil {
		return clienteRow{}, fmt.Errorf("ativo inválido (%q): %w", record[8], err)
	}

	if razaoSocial == "" || uf == "" {
		return clienteRow{}, fmt.Errorf("razao_social ou uf vazios")
	}

	return clienteRow{
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

// parseData tenta os layouts conhecidos de data_cadastro observados no CSV real.
func parseData(raw string) (time.Time, error) {
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

// parseAtivo converte 'S'/'N' (case-insensitive) para bool.
func parseAtivo(raw string) (bool, error) {
	switch strings.ToUpper(raw) {
	case "S":
		return true, nil
	case "N":
		return false, nil
	default:
		return false, fmt.Errorf("esperado 'S' ou 'N'")
	}
}

// resultadoUpsert agrega os contadores do upsertAll.
type resultadoUpsert struct {
	inserted      int // INSERT novo ou linha já existente sem alteração
	updated       int // UPDATE com alteração real
	conflitosCNPJ int // CNPJ já pertence a OUTRO cliente no banco (pulado)
	failed        int // demais erros de banco
}

// upsertAll grava todas as linhas no banco via INSERT ... ON DUPLICATE KEY UPDATE,
// usando cliente_id_origem como chave de idempotência.
//
// NEG-01: com o índice UNIQUE uq_clientes_cnpj, uma linha cujo CNPJ já
// pertence a OUTRO cliente no banco (ex.: cadastrado pela API) é pulada e
// contada em conflitosCNPJ. Sem essa checagem prévia, o ON DUPLICATE KEY
// UPDATE sobrescreveria silenciosamente o outro cliente (o conflito seria no
// índice de CNPJ, não na PK) ou falharia com 1062. O 1062 que ainda escapar
// (ex.: escrita concorrente) também é contado como conflito, sem abortar.
func upsertAll(db *sql.DB, rows []clienteRow) resultadoUpsert {
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

	donos, err := loadDonosCNPJ(db)
	if err != nil {
		log.Fatalf("importclientes: falha ao carregar CNPJs existentes: %v", err)
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatalf("importclientes: prepare falhou: %v", err)
	}
	defer stmt.Close()

	var res resultadoUpsert
	for _, row := range rows {
		if dono, ok := donos.dono(row.CNPJ); ok && dono != row.ClienteIDOrigem {
			log.Printf("importclientes: cliente_id_origem=%d pulado: CNPJ já pertence a outro cliente no banco (cliente_id_origem=%d)",
				row.ClienteIDOrigem, dono)
			res.conflitosCNPJ++
			continue
		}

		result, err := stmt.Exec(
			row.ClienteIDOrigem, row.CNPJ, row.RazaoSocial, row.Segmento,
			row.Cidade, row.UF, row.Bairro, row.DataCadastro.Format("2006-01-02"), row.Ativo,
		)
		if err != nil {
			if isDuplicateCNPJ(err) {
				log.Printf("importclientes: cliente_id_origem=%d pulado: CNPJ duplicado (1062 uq_clientes_cnpj)", row.ClienteIDOrigem)
				res.conflitosCNPJ++
				continue
			}
			log.Printf("importclientes: erro no upsert de cliente_id_origem=%d: %v", row.ClienteIDOrigem, err)
			res.failed++
			continue
		}
		donos.gravar(row.ClienteIDOrigem, row.CNPJ)

		// Com clientFoundRows=true no DSN (BUG-04), o MySQL devolve, via ON
		// DUPLICATE KEY UPDATE: 1 = INSERT novo OU linha já existente sem
		// alteração; 2 = UPDATE com alteração real. Por isso o contador
		// "inseridos" é rotulado como inseridos_ou_inalterados.
		affected, _ := result.RowsAffected()
		switch affected {
		case 1:
			res.inserted++
		default:
			// 2 = UPDATE com alteração; 0 não ocorre com clientFoundRows=true
			// (mantido por robustez).
			res.updated++
		}
	}
	return res
}

// donosCNPJ mantém, em memória, quem é o dono de cada CNPJ no banco durante
// a importação (cnpj → cliente_id_origem e o inverso, para liberar o CNPJ
// antigo quando uma linha troca de CNPJ).
type donosCNPJ struct {
	porCNPJ map[string]int64
	porID   map[int64]string
}

func (d donosCNPJ) dono(cnpj string) (int64, bool) {
	id, ok := d.porCNPJ[cnpj]
	return id, ok
}

func (d donosCNPJ) gravar(id int64, cnpj string) {
	if antigo, ok := d.porID[id]; ok && antigo != cnpj {
		delete(d.porCNPJ, antigo)
	}
	d.porID[id] = cnpj
	d.porCNPJ[cnpj] = id
}

// loadDonosCNPJ carrega cnpj → cliente_id_origem de todos os clientes do banco.
func loadDonosCNPJ(db *sql.DB) (donosCNPJ, error) {
	d := donosCNPJ{porCNPJ: map[string]int64{}, porID: map[int64]string{}}
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
		d.gravar(id, cnpj)
	}
	return d, rows.Err()
}

// findProjectRoot sobe a árvore de diretórios a partir do cwd até achar a
// raiz do repositório (identificada por apis/shared/go.mod).
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "apis", "shared", "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("não foi possível localizar apis/shared/go.mod a partir de %s", dir)
}

// loadEnvFromCwd tenta carregar o .env da raiz do projeto subindo diretórios
// a partir do working directory. Não retorna erro — se não achar, segue sem .env.
func loadEnvFromCwd() {
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
				_ = godotenv.Load(filepath.Join(dir, ".env"))
				return
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
}
