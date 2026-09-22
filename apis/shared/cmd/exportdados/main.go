// Command exportdados exporta as tabelas do banco de volta para arquivos CSV
// no mesmo formato/estrutura de dados/ (crm/ e erp/), mas lendo os dados
// atuais do banco (não dos CSVs originais).
//
// Uso:
//
//	cd apis/shared && go run ./cmd/exportdados
//	cd apis/shared && go run ./cmd/exportdados -out=/caminho/alternativo/export
//
// Também disponível via Makefile na raiz do projeto:
//
//	make db-export
//
// Comportamento:
//  1. Carrega .env da raiz do projeto (mesmo mecanismo dos demais comandos).
//  2. Conecta no banco configurado em config.Load().
//  3. Para cada tabela, roda uma query que reconstrói as colunas exatamente
//     como no CSV de origem (mesmos nomes/ordem de dados/crm|erp/*.csv) e
//     grava um arquivo CSV em <out>/crm/*.csv e <out>/erp/*.csv.
//  4. -out (default: "export" na raiz do repositório).
package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"github.com/rotaperfumes/shared/config"
)

// tableExport descreve uma exportação: arquivo de destino (relativo à pasta
// de export), header do CSV (na mesma ordem/nome do CSV de origem em dados/)
// e a query SQL que produz as colunas nessa mesma ordem.
type tableExport struct {
	relPath string
	header  []string
	query   string
}

var exports = []tableExport{
	{
		relPath: filepath.Join("crm", "clientes.csv"),
		header:  []string{"cliente_id", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro", "data_cadastro", "ativo"},
		query: `
			SELECT cliente_id_origem, cnpj, razao_social, segmento, cidade, uf,
			       COALESCE(bairro, ''), DATE_FORMAT(data_cadastro, '%Y-%m-%d'),
			       IF(ativo = 1, 'S', 'N')
			FROM clientes
			ORDER BY cliente_id_origem`,
	},
	{
		relPath: filepath.Join("crm", "vendedores.csv"),
		header:  []string{"vendedor_id", "nome", "regiao", "uf", "data_admissao", "data_desligamento", "meta_mensal"},
		query: `
			SELECT id, nome, regiao, uf, DATE_FORMAT(data_admissao, '%Y-%m-%d'),
			       COALESCE(DATE_FORMAT(data_desligamento, '%Y-%m-%d'), ''), meta_mensal
			FROM vendedores
			ORDER BY id`,
	},
	{
		relPath: filepath.Join("crm", "carteira.csv"),
		header:  []string{"carteira_id", "cliente_id", "vendedor_id", "data_inicio", "data_fim"},
		query: `
			SELECT carteira_id_origem, cliente_id, vendedor_id,
			       DATE_FORMAT(data_inicio, '%Y-%m-%d'),
			       COALESCE(DATE_FORMAT(data_fim, '%Y-%m-%d'), '')
			FROM carteiras
			ORDER BY carteira_id_origem`,
	},
	{
		relPath: filepath.Join("crm", "oportunidades.csv"),
		header:  []string{"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa", "probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda"},
		query: `
			SELECT oportunidade_id, cliente_id, vendedor_id, origem,
			       DATE_FORMAT(data_abertura, '%Y-%m-%d'), etapa, probabilidade_pct, valor_estimado,
			       COALESCE(DATE_FORMAT(data_fechamento, '%Y-%m-%d'), ''),
			       COALESCE(CAST(ciclo_dias AS CHAR), ''),
			       COALESCE(motivo_perda, '')
			FROM oportunidades
			ORDER BY oportunidade_id`,
	},
	{
		relPath: filepath.Join("crm", "visitas.csv"),
		header:  []string{"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min"},
		query: `
			SELECT visita_id, cliente_id, vendedor_id, DATE_FORMAT(data_visita, '%Y-%m-%d'),
			       resultado, duracao_min
			FROM visitas
			ORDER BY visita_id`,
	},
	{
		relPath: filepath.Join("erp", "produtos.csv"),
		header:  []string{"sku", "descricao", "categoria", "marca", "nota_olfativa", "preco_tabela", "custo_unitario", "unidade", "ativo", "data_lancamento"},
		query: `
			SELECT sku, descricao, categoria, marca, COALESCE(nota_olfativa, ''),
			       preco_tabela, custo_unitario, unidade, IF(ativo = 1, 'S', 'N'),
			       COALESCE(DATE_FORMAT(data_lancamento, '%Y-%m-%d'), '')
			FROM produtos
			ORDER BY id`,
	},
	{
		relPath: filepath.Join("erp", "pedidos.csv"),
		header:  []string{"pedido_id", "cliente_id", "vendedor_id", "data_pedido", "canal", "status", "valor_total"},
		query: `
			SELECT pedido_id_origem, cliente_id, vendedor_id, DATE_FORMAT(data_pedido, '%Y-%m-%d'),
			       canal, status, valor_total
			FROM pedidos
			ORDER BY pedido_id_origem`,
	},
	{
		relPath: filepath.Join("erp", "itens_pedido.csv"),
		header:  []string{"item_id", "pedido_id", "sku", "quantidade", "preco_praticado", "desconto_pct", "valor_bruto"},
		query: `
			SELECT ip.item_id_origem, ip.pedido_id, p.sku, ip.quantidade,
			       ip.preco_praticado, ip.desconto_pct, ip.valor_bruto
			FROM itens_pedido ip
			JOIN produtos p ON p.id = ip.produto_id
			ORDER BY ip.item_id_origem`,
	},
	{
		relPath: filepath.Join("erp", "pagamentos.csv"),
		header:  []string{"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct", "valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento"},
		query: `
			SELECT pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido,
			       DATE_FORMAT(data_vencimento, '%Y-%m-%d'),
			       COALESCE(DATE_FORMAT(data_pagamento, '%Y-%m-%d'), ''),
			       status_pagamento
			FROM pagamentos
			ORDER BY pagamento_id`,
	},
}

func main() {
	outFlag := flag.String("out", "", "diretório de destino da exportação (default: export na raiz do projeto)")
	flag.Parse()

	loadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("exportdados: falha ao carregar config: %v", err)
	}

	outDir := resolveOutDir(*outFlag)
	log.Printf("exportdados: exportando para %s", outDir)

	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		log.Fatalf("exportdados: sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("exportdados: ping no banco falhou: %v", err)
	}

	for _, exp := range exports {
		dest := filepath.Join(outDir, exp.relPath)
		n, err := exportTable(db, exp, dest)
		if err != nil {
			log.Fatalf("exportdados: falha exportando %s: %v", dest, err)
		}
		log.Printf("exportdados: %s — %d linhas", dest, n)
	}

	log.Printf("exportdados: OK — %d arquivos exportados", len(exports))
}

// exportTable roda a query de um tableExport e grava o resultado em CSV,
// criando os diretórios intermediários necessários.
func exportTable(db *sql.DB, exp tableExport, destPath string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return 0, fmt.Errorf("criando diretório: %w", err)
	}

	rows, err := db.Query(exp.query)
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

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(exp.header); err != nil {
		return 0, fmt.Errorf("escrevendo header: %w", err)
	}

	values := make([]sql.NullString, len(cols))
	scanArgs := make([]any, len(cols))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	record := make([]string, len(cols))
	n := 0
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return n, fmt.Errorf("scan: %w", err)
		}
		for i, v := range values {
			record[i] = v.String
		}
		if err := w.Write(record); err != nil {
			return n, fmt.Errorf("escrevendo linha: %w", err)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return n, fmt.Errorf("iterando linhas: %w", err)
	}

	w.Flush()
	return n, w.Error()
}

// resolveOutDir decide o diretório final de export, na ordem:
// flag -out > default (export/ na raiz do repositório).
func resolveOutDir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	root, err := findProjectRoot()
	if err != nil {
		log.Fatalf("exportdados: não foi possível localizar a raiz do projeto: %v", err)
	}
	return filepath.Join(root, "export")
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
