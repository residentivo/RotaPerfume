// Package integracao é o ponto de entrada único dos testes de integração Go
// (SEC-50): lê INTEGRATION e DB_* do ambiente, recusa o banco de PRODUÇÃO
// ANTES de qualquer conexão (mesma regra do db-guard-producao do Makefile,
// sem válvula: integração nunca roda na produção) e só então carrega o TLS
// (SEC-29/SEC-42) e abre o pool.
//
// Uso nos testes (dos dois módulos):
//
//	db := integracao.Abrir(t)   // pula sem INTEGRATION=1/credenciais
//	cfg := integracao.Config(t) // só a guarda + Config, sem conectar
//
// Nenhum _test.go lê INTEGRATION direto (catraca em shared/tests/config).
package integracao

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/rotaperfumes/shared/config"
	shareddb "github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/vlog"
)

// ProducaoHostsLAN: IP/hostname do servidor de produção. É produção com
// DB_PORT vazio ou 3306 (a porta publicada com --dev-ip); o MariaDB de dev
// do CasaOS roda no mesmo host, porta 3307. Igual a PRODUCAO_HOSTS_LAN do
// Makefile (teste de paridade em shared/tests/config).
var ProducaoHostsLAN = []string{"192.168.168.106", "ivo-inspiron-15-3530"}

// ProducaoHostsRede: nome/IP do container de produção na rede docker; é
// produção em qualquer porta. Igual a PRODUCAO_HOSTS_REDE do Makefile.
var ProducaoHostsRede = []string{"rotaperfumes-mariadb", "172.30.42.2"}

// portaProducao é a porta do MariaDB de produção publicada na LAN.
const portaProducao = 3306

var (
	// ErrSemIntegracao: INTEGRATION diferente de "1" (o teste é pulado).
	ErrSemIntegracao = errors.New("integracao: defina INTEGRATION=1 para rodar os testes de integração")
	// ErrSemCredenciais: DB_USUARIO ou DB_SENHA vazio (SEC-11; o teste é pulado).
	ErrSemCredenciais = errors.New("integracao: defina DB_USUARIO/DB_SENHA (SEC-11: sem default)")
	// ErrBancoProducao: DB_HOST/DB_PORT apontam para o banco de produção.
	ErrBancoProducao = errors.New("integracao: banco de PRODUÇÃO recusado")
)

// ValidarNaoProducao aplica a regra do db-guard-producao: host (sem espaços
// nas pontas, minúsculo, sem "." final) em ProducaoHostsRede é produção em
// qualquer porta; em ProducaoHostsLAN, com porta vazia ou 3306 (comparada
// como número). Porta não numérica é erro.
func ValidarNaoProducao(host, porta string) error {
	h := normalizarHost(host)
	if contem(ProducaoHostsRede, h) {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_da_rede_docker_de_producao_recusado")
		return erroProducao(host, porta)
	}
	if !contem(ProducaoHostsLAN, h) {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_fora_da_producao_ok")
		return nil
	}
	p := strings.TrimSpace(porta)
	if p == "" {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_lan_sem_porta_recusado")
		return erroProducao(host, porta)
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão porta_nao_numerica_recusada")
		return fmt.Errorf("integracao: DB_PORT=%q não é numérico (SEC-50)", porta)
	}
	if n == portaProducao {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_lan_porta_%d_recusado", n)
		return erroProducao(host, porta)
	}
	vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_lan_outra_porta_ok (porta=%d)", n)
	return nil
}

// ConfigDoAmbiente monta a Config do banco a partir do ambiente, nesta
// ordem: INTEGRATION=1, credenciais, guarda de produção e, só então, o TLS
// (LoadDBTLS lê a CA do disco) e o SEC-42. Não conecta.
func ConfigDoAmbiente() (*config.Config, error) {
	vlog.Printf("integracao.go", "ConfigDoAmbiente", "verificando INTEGRATION=1")
	if os.Getenv("INTEGRATION") != "1" {
		return nil, ErrSemIntegracao
	}
	cfg := &config.Config{
		DBHost:    envOuPadrao("DB_HOST", "localhost"),
		DBPort:    envOuPadrao("DB_PORT", "3306"),
		DBName:    envOuPadrao("DB_NAME", "rotaperfumes"),
		DBUsuario: os.Getenv("DB_USUARIO"),
		DBSenha:   os.Getenv("DB_SENHA"),
	}
	vlog.Printf("integracao.go", "ConfigDoAmbiente", "verificando credenciais (usuario_preenchido=%t)", cfg.DBUsuario != "")
	if cfg.DBUsuario == "" || cfg.DBSenha == "" {
		return nil, ErrSemCredenciais
	}
	if err := ValidarNaoProducao(cfg.DBHost, cfg.DBPort); err != nil {
		return nil, err
	}
	vlog.Printf("integracao.go", "ConfigDoAmbiente", "carregando TLS do banco (LoadDBTLS)")
	tlsDB, err := config.LoadDBTLS()
	if err != nil {
		return nil, err
	}
	if err := config.ValidarDBTLSObrigatorio(cfg.DBHost, tlsDB); err != nil {
		return nil, err
	}
	cfg.DBTLS = tlsDB
	vlog.Printf("integracao.go", "ConfigDoAmbiente", "decisão config_ok (tls=%t)", tlsDB.Ativo())
	return cfg, nil
}

// Config devolve a Config dos testes de integração: sem INTEGRATION=1 ou
// sem credenciais, t.Skip; banco de produção ou qualquer outro erro, t.Fatal.
// Devolve nil só quando t.Skip/t.Fatal retornam (TB falso em teste).
func Config(t testing.TB) *config.Config {
	t.Helper()
	cfg, err := ConfigDoAmbiente()
	switch {
	case errors.Is(err, ErrSemIntegracao):
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MariaDB local)")
		return nil
	case errors.Is(err, ErrSemCredenciais):
		t.Skip("teste de integração: defina DB_USUARIO/DB_SENHA (SEC-11: sem default)")
		return nil
	case err != nil:
		t.Fatalf("teste de integração recusado: %s", err.Error())
		return nil
	}
	return cfg
}

// Abrir aplica Config e abre o pool (db.Open faz o Ping), fechado no
// t.Cleanup.
func Abrir(t testing.TB) *sql.DB {
	t.Helper()
	cfg := Config(t)
	if cfg == nil {
		return nil
	}
	conn, err := shareddb.Open(cfg.DSN())
	if err != nil {
		t.Fatalf("MariaDB de integração indisponível: %s", err.Error())
		return nil
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func erroProducao(host, porta string) error {
	return fmt.Errorf("%w: DB_HOST=%q DB_PORT=%q é o banco de PRODUÇÃO; teste de integração recusado antes de conectar (SEC-50). Use o dev do CasaOS (porta 3307) ou um MariaDB descartável no loopback", ErrBancoProducao, host, porta)
}

func normalizarHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func contem(lista []string, v string) bool {
	for _, item := range lista {
		if item == v {
			return true
		}
	}
	return false
}

// envOuPadrao: vazio = padrão (mesma regra do config.Load).
func envOuPadrao(chave, padrao string) string {
	if v := os.Getenv(chave); v != "" {
		return v
	}
	return padrao
}
