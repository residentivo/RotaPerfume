// Package clientesdedup centraliza a unificação de clientes com CNPJ
// duplicado no CSV de origem (dados/crm/clientes.csv) — NEG-01.
//
// Decisão do usuário (2026-09-25): clientes.cnpj é UNIQUE (uq_clientes_cnpj)
// e, para cada CNPJ repetido no CSV, fica a PRIMEIRA ocorrência (na ordem do
// arquivo, que é crescente por cliente_id — equivale ao MIN(cliente_id) usado
// por sql/19_alter_clientes_cnpj_unique.sql). As ocorrências seguintes são
// "cópias": o importclientes não as grava, e os importadores que referenciam
// o cliente pelo cliente_id do CSV (carteiras, pedidos, oportunidades,
// visitas) redirecionam a cópia para o id sobrevivente.
//
// O dígito verificador do CNPJ NÃO é validado aqui: os dados do CSV são
// fictícios e podem ter DV inválido (a validação de DV vale só para a API).
package clientesdedup

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

	"github.com/rotaperfumes/shared/cnpj"
)

// EnvCSVPath é a variável de ambiente que sobrescreve o caminho do CSV de
// clientes (a mesma usada pelo importclientes).
const EnvCSVPath = "CLIENTES_CSV_PATH"

// Registro é o mínimo necessário para unificar: id do CSV e CNPJ.
type Registro struct {
	ClienteID int64
	CNPJ      string
}

// Unificacao mapeia o cliente_id de cada cópia para o cliente_id
// sobrevivente (1ª ocorrência do mesmo CNPJ). Ids que não são cópia não
// aparecem no mapa.
type Unificacao map[int64]int64

// NormalizarCNPJ aplica a normalização central de CNPJ (shared/cnpj, NEG-02):
// remove a máscara (".", "/", "-" e espaços — o CSV mistura CNPJ puro,
// mascarado e com espaços em volta), converte para MAIÚSCULAS e exige o
// formato de 14 caracteres (12 em [0-9A-Z] + 2 DVs numéricos). ok=false se o
// valor tiver outro caractere ou formato errado. O DV NÃO é validado aqui.
// É a mesma regra usada pelo importclientes, para que a unificação e a
// gravação enxerguem o mesmo CNPJ.
func NormalizarCNPJ(raw string) (normalizado string, ok bool) {
	normalizado, ok = cnpj.Normalizar(raw)
	if !ok || !cnpj.FormatoValido(normalizado) {
		return "", false
	}
	return normalizado, true
}

// Calcular percorre os registros na ordem recebida e devolve o mapa de
// cópias → sobrevivente. CNPJ vazio ou fora do formato (após normalizar) é
// ignorado.
func Calcular(regs []Registro) Unificacao {
	primeiro := make(map[string]int64, len(regs))
	u := Unificacao{}
	for _, r := range regs {
		doc, ok := NormalizarCNPJ(r.CNPJ)
		if !ok {
			continue
		}
		sobrevivente, visto := primeiro[doc]
		if !visto {
			primeiro[doc] = r.ClienteID
			continue
		}
		if r.ClienteID != sobrevivente {
			u[r.ClienteID] = sobrevivente
		}
	}
	return u
}

// Canonico devolve o id sobrevivente de clienteID (ele mesmo se não for cópia).
func (u Unificacao) Canonico(clienteID int64) int64 {
	if s, ok := u[clienteID]; ok {
		return s
	}
	return clienteID
}

// AplicarAoLookup redireciona, no mapa de lookup (cliente_id do CSV → id em
// clientes) carregado do banco, cada cópia para o id do sobrevivente — desde
// que o sobrevivente exista no banco. Retorna quantas cópias foram
// redirecionadas.
func (u Unificacao) AplicarAoLookup(lookup map[int64]int64) int {
	n := 0
	for copia, sobrevivente := range u {
		if id, ok := lookup[sobrevivente]; ok {
			lookup[copia] = id
			n++
		}
	}
	return n
}

// Redirecionar carrega a unificação do CSV de clientes (CaminhoCSV) e a
// aplica ao lookup de clientes do importador (AplicarAoLookup), logando o
// total redirecionado com a tag do importador. Falha ao ler o CSV só gera
// aviso: o importador segue sem unificação (as cópias ficam sem cliente no
// lookup e são contadas como erro, como antes). Devolve a unificação lida.
func Redirecionar(tag, projectRoot string, lookup map[int64]int64) Unificacao {
	path := CaminhoCSV(projectRoot)
	u, err := CarregarDoCSV(path)
	if err != nil {
		log.Printf("%s: aviso: unificação de CNPJ duplicado não aplicada: %v", tag, err)
		return Unificacao{}
	}
	n := u.AplicarAoLookup(lookup)
	log.Printf("%s: %d cliente_id(s) de CNPJ duplicado redirecionados para o cliente sobrevivente (%d cópias no CSV de clientes)",
		tag, n, len(u))
	return u
}

// CaminhoCSV resolve o caminho do CSV de clientes: env CLIENTES_CSV_PATH ou
// <raiz do projeto>/dados/crm/clientes.csv.
func CaminhoCSV(projectRoot string) string {
	if v := os.Getenv(EnvCSVPath); v != "" {
		return v
	}
	return filepath.Join(projectRoot, "dados", "crm", "clientes.csv")
}

// CarregarDoCSV lê só as colunas cliente_id e cnpj do CSV de clientes e
// calcula a unificação. Linhas com cliente_id inválido ou CNPJ fora do
// formato (ver NormalizarCNPJ) são ignoradas (o importclientes também as
// descarta).
func CarregarDoCSV(path string) (Unificacao, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("clientesdedup: abrindo %s: %w", path, err)
	}
	defer f.Close()
	return LerRegistros(f)
}

// LerRegistros lê o CSV de clientes de rd (cabeçalho + linhas) e calcula a
// unificação. Mesmas regras de CarregarDoCSV, sem abrir arquivo.
func LerRegistros(rd io.Reader) (Unificacao, error) {
	r := csv.NewReader(rd)
	r.FieldsPerRecord = -1
	if _, err := r.Read(); err != nil {
		return nil, fmt.Errorf("clientesdedup: lendo cabeçalho: %w", err)
	}
	var regs []Registro
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || len(rec) < 2 {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(rec[0]), 10, 64)
		if err != nil {
			continue
		}
		doc, ok := NormalizarCNPJ(rec[1])
		if !ok {
			continue
		}
		regs = append(regs, Registro{ClienteID: id, CNPJ: doc})
	}
	return Calcular(regs), nil
}
