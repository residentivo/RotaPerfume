// Package integracao é o ponto de entrada único dos testes de integração Go
// (SEC-50): lê INTEGRATION e DB_* do ambiente, recusa o banco de PRODUÇÃO
// ANTES de qualquer conexão (mesma regra do db-guard-producao do Makefile,
// sem válvula: integração nunca roda na produção), aplica a allowlist da
// integração (SEC-54: loopback ou o CasaOS de dev na 3307, igual ao
// db-guard-integracao) e só então carrega o TLS (SEC-29/SEC-42) e abre o pool.
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
	"regexp"
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

// Permitidos (SEC-54): allowlist dos testes de integração, igual a
// INTEGRACAO_PERMITIDOS do Makefile (paridade testada em shared/tests/config).
// "host" vale em qualquer porta; "host:porta", só nessa porta; "127.0.0.0/8"
// é o loopback IPv4 (textual). O CasaOS de dev (192.168.168.106) só na 3307.
var Permitidos = []string{"localhost", "127.0.0.0/8", "::1", "192.168.168.106:3307"}

// portaProducao é a porta do MariaDB de produção publicada na LAN.
const portaProducao = 3306

var (
	// ErrSemIntegracao: INTEGRATION diferente de "1" (o teste é pulado).
	ErrSemIntegracao = errors.New("integracao: defina INTEGRATION=1 para rodar os testes de integração")
	// ErrSemCredenciais: DB_USUARIO ou DB_SENHA vazio (SEC-11; o teste é pulado).
	ErrSemCredenciais = errors.New("integracao: defina DB_USUARIO/DB_SENHA (SEC-11: sem default)")
	// ErrBancoProducao: DB_HOST/DB_PORT apontam para o banco de produção.
	ErrBancoProducao = errors.New("integracao: banco de PRODUÇÃO recusado")
	// ErrHostNaoPermitido (SEC-54): DB_HOST/DB_PORT fora de Permitidos.
	ErrHostNaoPermitido = errors.New("integracao: DB_HOST/DB_PORT fora da allowlist da integração")
	// ErrDBValorInvalido (SEC-63): DB_HOST, DB_NAME ou DB_USUARIO fora do charset
	// (entrariam no DSN e poderiam trocar o endereço real depois da guarda).
	ErrDBValorInvalido = errors.New("integracao: valor inválido em variável DB_*")
	// ErrPortaInvalida (SEC-57): DB_PORT fora de "+"? e dígitos (espaços
	// inclusive; não há TrimSpace), em qualquer host.
	ErrPortaInvalida = errors.New("integracao: porta recusada")
)

// ValidarNaoProducao aplica a regra do db-guard-producao (PRODUCAO_DETECTA
// do Makefile; paridade testada em shared/tests/config), sem DNS:
//   - porta não numérica (fora de "+"? e dígitos, espaços inclusive) é
//     ErrPortaInvalida, em qualquer host (SEC-57);
//   - host normalizado (normalizarHost) com ":" que não seja "::1" é
//     produção (fail-closed para literais IPv6 que não são loopback);
//   - host IPv4 numérico fora da forma canônica a.b.c.d (decimal, hex,
//     octal, curta) é produção, sem válvula (SEC-56);
//   - host em ProducaoHostsRede é produção em qualquer porta; em
//     ProducaoHostsLAN, com porta vazia ou 3306 (comparada como número).
func ValidarNaoProducao(host, porta string) error {
	p, ok := normalizarPorta(porta)
	if !ok {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão porta_nao_numerica_recusada")
		return erroPortaInvalida(porta)
	}
	h, _ := normalizarHost(host)
	if ipv4NaoCanonico(h) {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão ipv4_nao_canonico_recusado")
		return fmt.Errorf("%w: SEC-56: DB_HOST=%q e IPv4 em forma numerica nao canonica (decimal/hex/octal/curta): use a.b.c.d", ErrBancoProducao, host)
	}
	if strings.Contains(h, ":") && h != "::1" {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão literal_ipv6_nao_loopback_recusado")
		return erroProducao(host, porta)
	}
	if contem(ProducaoHostsRede, h) {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_da_rede_docker_de_producao_recusado")
		return erroProducao(host, porta)
	}
	if !contem(ProducaoHostsLAN, h) {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_fora_da_producao_ok")
		return nil
	}
	if p == "" || p == strconv.Itoa(portaProducao) {
		vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_lan_porta_de_producao_recusado (porta=%q)", p)
		return erroProducao(host, porta)
	}
	vlog.Printf("integracao.go", "ValidarNaoProducao", "decisão host_lan_outra_porta_ok (porta=%s)", p)
	return nil
}

// ValidarPermitidoIntegracao (SEC-54) aplica a allowlist da integração
// (INTEGRACAO_PERMITIDOS do db-guard-integracao no Makefile): só os
// destinos de Permitidos passam. Porta vazia conta como 3306. Host que
// só bate depois de tirar o sufixo ".local" (nome mDNS, pode resolver para
// qualquer máquina) não passa.
func ValidarPermitidoIntegracao(host, porta string) error {
	p, ok := normalizarPorta(porta)
	if !ok {
		vlog.Printf("integracao.go", "ValidarPermitidoIntegracao", "decisão porta_nao_numerica_recusada")
		return fmt.Errorf("%w; %w", erroNaoPermitido(host, porta), erroPortaInvalida(porta))
	}
	if p == "" {
		p = strconv.Itoa(portaProducao)
	}
	h, eraLocal := normalizarHost(host)
	if eraLocal {
		vlog.Printf("integracao.go", "ValidarPermitidoIntegracao", "decisão sufixo_local_recusado")
		return erroNaoPermitido(host, porta)
	}
	for _, e := range Permitidos {
		if permitidoCasa(e, h, p) {
			vlog.Printf("integracao.go", "ValidarPermitidoIntegracao", "decisão permitido (regra=%s)", e)
			return nil
		}
	}
	vlog.Printf("integracao.go", "ValidarPermitidoIntegracao", "decisão fora_da_allowlist_recusado")
	return erroNaoPermitido(host, porta)
}

// permitidoCasa: "127.0.0.0/8" casa 127.x.x.x com octetos canônicos
// (textual, como o LOOPBACK_V4 do Makefile; SEC-56); entrada com um só ":" é host:porta; o resto é o host em
// qualquer porta ("::1", "localhost").
func permitidoCasa(entrada, host, porta string) bool {
	switch {
	case entrada == "127.0.0.0/8":
		return reLoopbackV4.MatchString(host)
	case strings.Count(entrada, ":") == 1:
		return entrada == host+":"+porta
	default:
		return entrada == host
	}
}

// Charsets do SEC-63, os mesmos do Makefile (paridade testada em
// shared/tests/config). O "-" inicial é recusado: viraria opção no cliente mariadb.
var (
	reFormaHost    = regexp.MustCompile(`^[A-Za-z0-9._:\[\]][A-Za-z0-9._:\[\]-]*$`)
	reFormaNome    = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
	reFormaUsuario = regexp.MustCompile(`^[A-Za-z0-9_.][A-Za-z0-9_.-]*$`)
)

// ValidarFormaDB (SEC-63) confere host, nome do banco e usuário antes da guarda:
// o DSN é montado por texto e o driver separa pelo último "@" e pelo último "/",
// então um DB_NAME como x@tcp(host:3306)/banco mudaria o endereço real depois de
// a guarda aprovar o DB_HOST. A mensagem cita só a variável, nunca o valor.
func ValidarFormaDB(host, nome, usuario string) error {
	vlog.Printf("integracao.go", "ValidarFormaDB", "conferindo charset de DB_HOST, DB_NAME e DB_USUARIO")
	switch {
	case !reFormaHost.MatchString(host):
		return fmt.Errorf("%w: DB_HOST (SEC-63: só A-Z a-z 0-9 . _ - : [ ], sem \"-\" inicial)", ErrDBValorInvalido)
	case !reFormaNome.MatchString(nome):
		return fmt.Errorf("%w: DB_NAME (SEC-63: só A-Z a-z 0-9 _)", ErrDBValorInvalido)
	case !reFormaUsuario.MatchString(usuario):
		return fmt.Errorf("%w: DB_USUARIO (SEC-63: só A-Z a-z 0-9 _ . -, sem \"-\" inicial)", ErrDBValorInvalido)
	}
	return nil
}

// ConfigDoAmbiente monta a Config do banco a partir do ambiente, nesta
// ordem: INTEGRATION=1, credenciais, forma dos valores (SEC-63), guarda de produção, allowlist da
// integração (SEC-54) e, só então, o TLS
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
	if err := ValidarFormaDB(cfg.DBHost, cfg.DBName, cfg.DBUsuario); err != nil {
		return nil, err
	}
	if err := ValidarNaoProducao(cfg.DBHost, cfg.DBPort); err != nil {
		return nil, err
	}
	if err := ValidarPermitidoIntegracao(cfg.DBHost, cfg.DBPort); err != nil {
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

func erroPortaInvalida(porta string) error {
	return fmt.Errorf("%w: DB_PORT=%q inválida: não é numérico (SEC-57: só dígitos, '+' opcional, sem espaços)", ErrPortaInvalida, porta)
}

func erroNaoPermitido(host, porta string) error {
	return fmt.Errorf("%w: DB_HOST=%q DB_PORT=%q fora de %s (SEC-54); teste de integração recusado antes de conectar", ErrHostNaoPermitido, host, porta, strings.Join(Permitidos, " "))
}

// octetoV4 (SEC-56): octeto IPv4 canônico (0-255, sem zero à esquerda),
// igual ao OCTETO_V4 do Makefile.
const octetoV4 = `(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])`

var (
	reLoopbackV4  = regexp.MustCompile(`^127(\.` + octetoV4 + `){3}$`)
	reIPv4Mapeado = regexp.MustCompile(`^(?:::ffff:|0:0:0:0:0:ffff:)([0-9]{1,3}(?:\.[0-9]{1,3}){3})$`)
	rePorta       = regexp.MustCompile(`^\+?[0-9]+$`)
	// SEC-56: IPV4_NUMERICO e IPV4_CANONICO do Makefile.
	reIPv4Numerico = regexp.MustCompile(`^(0x[0-9a-f]*|[0-9]+)(\.(0x[0-9a-f]*|[0-9]+))*$`)
	reIPv4Canonico = regexp.MustCompile(`^` + octetoV4 + `(\.` + octetoV4 + `){3}$`)
)

// ipv4NaoCanonico (SEC-56): h (já normalizado, minúsculo) é uma forma
// numérica de IPv4 que o resolver aceitaria (3232278634, 0xc0a8a86a,
// 0300.0250.0250.0152, 192.168.43114, 127.1) mas não é a.b.c.d canônico.
func ipv4NaoCanonico(h string) bool {
	return reIPv4Numerico.MatchString(h) && !reIPv4Canonico.MatchString(h)
}

// normalizarHost (SEC-54) é a mesma normalização textual do NORMALIZA_HOST
// do Makefile, sem DNS: espaços nas pontas, minúsculas, "[" e "]" das
// pontas, todos os "." finais, um sufixo ".local" e IPv4 mapeado em IPv6
// (::ffff:a.b.c.d e 0:0:0:0:0:ffff:a.b.c.d viram a.b.c.d). eraLocal informa
// se o sufixo ".local" foi tirado.
func normalizarHost(host string) (h string, eraLocal bool) {
	h = strings.ToLower(strings.TrimSpace(host))
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")
	h = strings.TrimRight(h, ".")
	if sem, ok := strings.CutSuffix(h, ".local"); ok {
		h, eraLocal = sem, true
	}
	if m := reIPv4Mapeado.FindStringSubmatch(h); m != nil {
		h = m[1]
	}
	return h, eraLocal
}

// normalizarPorta (SEC-54): vazia fica vazia; senão exige "+"? e dígitos e
// tira o "+" e os zeros à esquerda (03306 e +3306 viram 3306), igual ao
// PRODUCAO_DETECTA do Makefile. ok=false para porta não numérica. SEC-57:
// sem TrimSpace ("3307 " e "  " são inválidas, como no Makefile).
func normalizarPorta(porta string) (p string, ok bool) {
	p = porta
	if p == "" {
		return "", true
	}
	if !rePorta.MatchString(p) {
		return "", false
	}
	p = strings.TrimLeft(strings.TrimPrefix(p, "+"), "0")
	if p == "" {
		p = "0"
	}
	return p, true
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
