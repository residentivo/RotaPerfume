// Package exportdados implementa o comando cmd/exportdados: exporta as
// tabelas do banco de volta para arquivos CSV no mesmo formato/estrutura de
// dados/ (crm/ e erp/), mas lendo os dados atuais do banco (não dos CSVs
// originais).
//
// Uso (via comando):
//
//	cd apis/shared && go run ./cmd/exportdados
//	cd apis/shared && go run ./cmd/exportdados -out=/caminho/alternativo/export
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-export
//
// Comportamento:
//  1. O comando carrega .env da raiz do projeto (cmdutil.LoadEnvFromCwd).
//  2. Conecta no banco configurado em config.Load().
//  3. Para cada tabela, roda uma query que reconstrói as colunas exatamente
//     como no CSV de origem (mesmos nomes/ordem de dados/crm|erp/*.csv) e
//     grava um arquivo CSV em <out>/crm/*.csv e <out>/erp/*.csv.
//  4. -out (default: "export" na raiz do repositório).
package exportdados

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/rotaperfumes/shared/cmdutil"
)

// Tag prefixa os logs e as mensagens de erro do comando.
const Tag = "exportdados"

// Querier é o subconjunto de *sql.DB usado para ler as tabelas (satisfeito
// por *sql.DB, *sql.Tx e pelo *sql.DB do sqlmock).
type Querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// Options configura uma execução do export.
type Options struct {
	// OutFlag é o valor da flag -out (vazio = export/ na raiz do projeto).
	OutFlag string
}

// Run resolve o diretório de destino, abre o banco via open e exporta todas
// as tabelas de Exports. Erros fatais são devolvidos sem o prefixo Tag.
func Run(opts Options, open cmdutil.Opener) error {
	outDir, err := ResolveOutDir(opts.OutFlag)
	if err != nil {
		return err
	}
	log.Printf("exportdados: exportando para %s", outDir)

	db, err := open()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping no banco falhou: %w", err)
	}

	return ExportAll(db, outDir, Exports)
}

// ExportAll exporta cada tabela de exps para outDir/<RelPath>, parando no
// primeiro erro.
func ExportAll(db Querier, outDir string, exps []TableExport) error {
	for _, exp := range exps {
		dest := filepath.Join(outDir, exp.RelPath)
		n, err := ExportTable(db, exp, dest)
		if err != nil {
			return fmt.Errorf("falha exportando %s: %w", dest, err)
		}
		log.Printf("exportdados: %s — %d linhas", dest, n)
	}

	log.Printf("exportdados: OK — %d arquivos exportados", len(exps))
	return nil
}

// TableExport descreve uma exportação: arquivo de destino (relativo à pasta
// de export), header do CSV (na mesma ordem/nome do CSV de origem em dados/)
// e a query SQL que produz as colunas nessa mesma ordem.
type TableExport struct {
	RelPath string
	Header  []string
	Query   string
}

// Exports lista as tabelas exportadas, na ordem de gravação.
var Exports = []TableExport{
	{
		RelPath: filepath.Join("crm", "clientes.csv"),
		Header:  []string{"cliente_id", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro", "ativo"},
		Query: `
			SELECT cliente_id_origem, cnpj, razao_social, segmento, cidade, uf,
			       COALESCE(bairro, ''), DATE_FORMAT(data_cadastro, '%Y-%m-%d'),
			       IF(ativo = 1, 'S', 'N')
			FROM clientes
			ORDER BY cliente_id_origem`,
	},
	{
		RelPath: filepath.Join("crm", "vendedores.csv"),
		Header:  []string{"vendedor_id", "nome", "regiao", "uf", "data_admissao", "data_desligamento", "meta_mensal"},
		Query: `
			SELECT id, nome, regiao, uf, DATE_FORMAT(data_admissao, '%Y-%m-%d'),
			       COALESCE(DATE_FORMAT(data_desligamento, '%Y-%m-%d'), ''), meta_mensal
			FROM vendedores
			ORDER BY id`,
	},
	{
		RelPath: filepath.Join("crm", "carteira.csv"),
		Header:  []string{"carteira_id", "cliente_id", "vendedor_id", "data_inicio", "data_fim"},
		Query: `
			SELECT carteira_id_origem, cliente_id, vendedor_id,
			       DATE_FORMAT(data_inicio, '%Y-%m-%d'),
			       COALESCE(DATE_FORMAT(data_fim, '%Y-%m-%d'), '')
			FROM carteiras
			ORDER BY carteira_id_origem`,
	},
	{
		RelPath: filepath.Join("crm", "oportunidades.csv"),
		Header:  []string{"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa", "probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda"},
		Query: `
			SELECT oportunidade_id, cliente_id, vendedor_id, origem,
			       DATE_FORMAT(data_abertura, '%Y-%m-%d'), etapa, probabilidade_pct, valor_estimado,
			       COALESCE(DATE_FORMAT(data_fechamento, '%Y-%m-%d'), ''),
			       COALESCE(CAST(ciclo_dias AS CHAR), ''),
			       COALESCE(motivo_perda, '')
			FROM oportunidades
			ORDER BY oportunidade_id`,
	},
	{
		RelPath: filepath.Join("crm", "visitas.csv"),
		Header:  []string{"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min"},
		Query: `
			SELECT visita_id, cliente_id, vendedor_id, DATE_FORMAT(data_visita, '%Y-%m-%d'),
			       resultado, duracao_min
			FROM visitas
			ORDER BY visita_id`,
	},
	{
		RelPath: filepath.Join("erp", "produtos.csv"),
		Header:  []string{"sku", "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela", "custo_unitario", "unidade", "ativo", "data_lancamento"},
		Query: `
			SELECT sku, descricao, categoria, marca, COALESCE(nota_olfativa, ''),
			       preco_tabela, custo_unitario, unidade, IF(ativo = 1, 'S', 'N'),
			       COALESCE(DATE_FORMAT(data_lancamento, '%Y-%m-%d'), '')
			FROM produtos
			ORDER BY id`,
	},
	{
		RelPath: filepath.Join("erp", "pedidos.csv"),
		Header:  []string{"pedido_id", "cliente_id", "vendedor_id", "data_pedido", "canal", "status", "valor_total"},
		Query: `
			SELECT pedido_id_origem, cliente_id, vendedor_id, DATE_FORMAT(data_pedido, '%Y-%m-%d'),
			       canal, status, valor_total
			FROM pedidos
			ORDER BY pedido_id_origem`,
	},
	{
		RelPath: filepath.Join("erp", "itens_pedido.csv"),
		Header:  []string{"item_id", "pedido_id", "sku", "quantidade", "preco_praticado", "desconto_pct", "valor_bruto"},
		Query: `
			SELECT ip.item_id_origem, ip.pedido_id, p.sku, ip.quantidade,
			       ip.preco_praticado, ip.desconto_pct, ip.valor_bruto
			FROM itens_pedido ip
			JOIN produtos p ON p.id = ip.produto_id
			ORDER BY ip.item_id_origem`,
	},
	{
		RelPath: filepath.Join("erp", "pagamentos.csv"),
		Header:  []string{"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct", "valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento"},
		Query: `
			SELECT pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido,
			       DATE_FORMAT(data_vencimento, '%Y-%m-%d'),
			       COALESCE(DATE_FORMAT(data_pagamento, '%Y-%m-%d'), ''),
			       status_pagamento
			FROM pagamentos
			ORDER BY pagamento_id`,
	},
}

// ExportTable roda a query de um TableExport e grava o resultado em CSV,
// criando os diretórios intermediários necessários.
func ExportTable(db Querier, exp TableExport, destPath string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, fmt.Errorf("criando diretório: %w", err)
	}

	rows, err := db.Query(exp.Query)
	if err != nil {
		return 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return 0, fmt.Errorf("columns: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return 0, fmt.Errorf("criando arquivo: %w", err)
	}
	defer f.Close()

	return WriteCSV(f, exp.Header, rows, len(cols))
}

// WriteCSV grava o header e todas as linhas de rows (com ncols colunas) em w
// no formato CSV. Valores NULL viram string vazia. Devolve quantas linhas
// (sem o header) foram escritas.
func WriteCSV(w io.Writer, header []string, rows *sql.Rows, ncols int) (int, error) {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write(header); err != nil {
		return 0, fmt.Errorf("escrevendo header: %w", err)
	}

	values := make([]sql.NullString, ncols)
	scanArgs := make([]any, ncols)
	for i := range values {
		scanArgs[i] = &values[i]
	}

	record := make([]string, ncols)
	n := 0
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return n, fmt.Errorf("scan: %w", err)
		}
		for i, v := range values {
			record[i] = v.String
		}
		if err := cw.Write(record); err != nil {
			return n, fmt.Errorf("escrevendo linha: %w", err)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return n, fmt.Errorf("iterando linhas: %w", err)
	}

	cw.Flush()
	return n, cw.Error()
}

// ResolveOutDir decide o diretório final de export, na ordem:
// flag -out > default (export/ na raiz do repositório).
func ResolveOutDir(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	root, err := cmdutil.FindProjectRoot()
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a raiz do projeto: %w", err)
	}
	return filepath.Join(root, "export"), nil
}
