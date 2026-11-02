package config

// TLS da conexão Go ↔ MariaDB (SEC-29).
//
// DB_TLS_CA (caminho de um PEM com a CA que assinou o certificado do servidor)
// liga o TLS: o PEM é carregado num x509.CertPool, registrado no driver com
// mysql.RegisterTLSConfig(DBTLSConfigName, ...) e o DSN ganha tls=custom.
// Sem DB_TLS_CA o DSN fica como antes (dev local sem TLS).
//
// DB_TLS_SERVER_NAME (opcional) é o nome conferido no certificado quando o
// host do DSN não está no SAN. Vazio = o próprio DB_HOST (o driver preenche o
// ServerName com o host do DSN). Em produção (DEPLOY-06) DB_HOST é o nome do
// container na rede rotaperfumes-db (rotaperfumes-mariadb), que já está no SAN.
//
// TST-12: o cliente exige TLS 1.3 (MinVersion). O servidor aceita 1.2 e 1.3;
// servidor que só fala 1.2 (ou menos) é recusado no handshake.
//
// SEC-42: DB_TLS_REQUIRED (padrão true) torna o TLS obrigatório quando DB_HOST
// não é loopback literal (ver ValidarDBTLSObrigatorio).

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/rotaperfumes/shared/projroot"
	"github.com/rotaperfumes/shared/vlog"
)

// DBTLSConfigName é o nome da config TLS registrada no driver (tls=custom).
const DBTLSConfigName = "custom"

// ErrDBTLSCA: DB_TLS_CA aponta para arquivo ilegível ou sem certificado
// válido. Embrulhado com o caminho resolvido e a causa.
var ErrDBTLSCA = errors.New("config: DB_TLS_CA inválido (esperado caminho de um PEM com o certificado da CA do MariaDB)")

// ErrDBTLSServerName: DB_TLS_SERVER_NAME com espaço, porta, esquema, caminho
// ou curinga, ou definido sem DB_TLS_CA (não teria efeito: falha fechada).
var ErrDBTLSServerName = errors.New("config: DB_TLS_SERVER_NAME inválido (hostname ou IP, sem porta/esquema; exige DB_TLS_CA)")

// ErrDBTLSObrigatorio (SEC-42): DB_HOST fora do loopback sem DB_TLS_CA com
// DB_TLS_REQUIRED=true (padrão). Falha fechada: a senha não sai em texto claro.
var ErrDBTLSObrigatorio = errors.New("config: DB_TLS_CA é obrigatório quando DB_HOST não é loopback (localhost, 127.0.0.0/8, ::1); defina DB_TLS_CA ou, só se souber o risco, DB_TLS_REQUIRED=false")

// ErrDBTLSRequired (SEC-42): DB_TLS_REQUIRED com valor diferente de true/false.
var ErrDBTLSRequired = errors.New("config: DB_TLS_REQUIRED inválido (esperado true ou false)")

// DBTLS é a configuração de TLS do banco já validada e registrada no driver.
// Valor zero = TLS desligado.
type DBTLS struct {
	// CAFile é o caminho absoluto do PEM da CA (vazio = TLS desligado).
	CAFile string
	// ServerName é o nome conferido no certificado (vazio = DB_HOST).
	ServerName string
}

// Ativo informa se a conexão com o banco usa TLS.
func (t DBTLS) Ativo() bool { return t.CAFile != "" }

// parametroDSN é o sufixo do DSN: "&tls=custom" com TLS, "" sem.
func (t DBTLS) parametroDSN() string {
	if !t.Ativo() {
		return ""
	}
	return "&tls=" + DBTLSConfigName
}

// Descricao resume o estado para log de startup (sem segredo: só o caminho
// da CA, que é pública, e o ServerName).
func (t DBTLS) Descricao() string {
	if !t.Ativo() {
		return "desligado (DB_TLS_CA vazio)"
	}
	nome := t.ServerName
	if nome == "" {
		nome = "DB_HOST"
	}
	return fmt.Sprintf("ligado (ca=%s, server_name=%s, min=TLS1.3)", t.CAFile, nome)
}

// LoadDBTLS lê DB_TLS_CA/DB_TLS_SERVER_NAME, carrega a CA e registra a config
// TLS no driver. Erro claro (ErrDBTLSCA/ErrDBTLSServerName) se a CA for
// ilegível/inválida ou o ServerName for inválido. Exposto para ferramentas e
// testes de integração que montam a Config à mão.
func LoadDBTLS() (DBTLS, error) {
	vlog.Printf("db_tls.go", "LoadDBTLS", "lendo DB_TLS_CA e DB_TLS_SERVER_NAME com trim")
	caRaw := strings.TrimSpace(os.Getenv("DB_TLS_CA"))
	serverName := strings.TrimSpace(os.Getenv("DB_TLS_SERVER_NAME"))

	if caRaw == "" {
		if serverName != "" {
			vlog.Printf("db_tls.go", "LoadDBTLS", "decisão server_name_sem_ca_recusado")
			return DBTLS{}, ErrDBTLSServerName
		}
		vlog.Printf("db_tls.go", "LoadDBTLS", "decisão tls_desligado (DB_TLS_CA vazio)")
		return DBTLS{}, nil
	}
	if !serverNameValido(serverName) {
		vlog.Printf("db_tls.go", "LoadDBTLS", "decisão server_name_invalido_recusado")
		return DBTLS{}, ErrDBTLSServerName
	}

	caFile := resolverCaminhoCA(caRaw)
	vlog.Printf("db_tls.go", "LoadDBTLS", "carregando CA do banco de %s", caFile)
	pool, err := carregarPoolCA(caFile)
	if err != nil {
		return DBTLS{}, err
	}

	vlog.Printf("db_tls.go", "LoadDBTLS", "registrando config TLS %q no driver", DBTLSConfigName)
	if err := mysql.RegisterTLSConfig(DBTLSConfigName, &tls.Config{
		RootCAs:    pool,
		ServerName: serverName,
		MinVersion: tls.VersionTLS13,
	}); err != nil {
		return DBTLS{}, fmt.Errorf("config: registrando TLS do banco: %w", err)
	}
	vlog.Printf("db_tls.go", "LoadDBTLS", "decisão tls_ligado")
	return DBTLS{CAFile: caFile, ServerName: serverName}, nil
}

// resolverCaminhoCA: absoluto fica como está; relativo é resolvido a partir
// da raiz do repositório (pasta com backend/shared/go.mod) quando o cwd está
// dentro dele, senão a partir do cwd. Assim "deploy/ca-cert.pem" do .env vale
// para make dev-api, go test em qualquer pacote e os comandos de shared.
func resolverCaminhoCA(raw string) string {
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	if raiz, err := projroot.Find(); err == nil {
		return filepath.Join(raiz, raw)
	}
	if abs, err := filepath.Abs(raw); err == nil {
		return abs
	}
	return filepath.Clean(raw)
}

// carregarPoolCA lê o PEM e exige ao menos um bloco CERTIFICATE válido;
// blocos de outro tipo (ex.: chave privada colada por engano) são recusados.
func carregarPoolCA(caminho string) (*x509.CertPool, error) {
	dados, err := os.ReadFile(caminho)
	if err != nil {
		// O erro do os já traz o caminho (ex.: "open <caminho>: no such file").
		return nil, fmt.Errorf("%w: %v", ErrDBTLSCA, err)
	}
	pool := x509.NewCertPool()
	total := 0
	for resto := dados; ; {
		var bloco *pem.Block
		bloco, resto = pem.Decode(resto)
		if bloco == nil {
			break
		}
		if bloco.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("%w: %s: bloco PEM %q não é certificado", ErrDBTLSCA, caminho, bloco.Type)
		}
		cert, err := x509.ParseCertificate(bloco.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %v", ErrDBTLSCA, caminho, err)
		}
		pool.AddCert(cert)
		total++
	}
	if total == 0 {
		return nil, fmt.Errorf("%w: %s: nenhum certificado PEM encontrado", ErrDBTLSCA, caminho)
	}
	vlog.Printf("db_tls.go", "carregarPoolCA", "certificados carregados na pool da CA: %d", total)
	return pool, nil
}

// serverNameValido: vazio (usa DB_HOST), IP ou hostname sem porta, esquema,
// caminho, curinga nem espaço.
func serverNameValido(s string) bool {
	if s == "" || net.ParseIP(s) != nil {
		return true
	}
	return hostnameValido(s)
}

// ValidarDBTLSObrigatorio aplica o SEC-42: com DB_TLS_REQUIRED=true (padrão),
// DB_HOST fora do loopback exige TLS ligado (ErrDBTLSObrigatorio).
// DB_TLS_REQUIRED=false explícito libera, com log AVISO. Valor diferente de
// true/false é erro (ErrDBTLSRequired), mesmo com host loopback.
func ValidarDBTLSObrigatorio(host string, t DBTLS) error {
	vlog.Printf("db_tls.go", "ValidarDBTLSObrigatorio", "lendo DB_TLS_REQUIRED (padrão true) e verificando erro")
	obrigatorio, err := parseDBTLSRequired()
	if err != nil {
		return err
	}
	if t.Ativo() || HostLoopback(host) {
		vlog.Printf("db_tls.go", "ValidarDBTLSObrigatorio", "decisão ok (tls_ativo=%t, loopback=%t)", t.Ativo(), HostLoopback(host))
		return nil
	}
	if obrigatorio {
		vlog.Printf("db_tls.go", "ValidarDBTLSObrigatorio", "decisão host_remoto_sem_tls_recusado")
		return fmt.Errorf("%w (DB_HOST=%q)", ErrDBTLSObrigatorio, host)
	}
	log.Printf("[config] AVISO: DB_TLS_REQUIRED=false: conexão com o banco SEM TLS para DB_HOST=%q (fora do loopback); usuário e senha trafegam em texto claro", host)
	return nil
}

// parseDBTLSRequired: vazio = true; só "true"/"false" (com trim) são aceitos.
func parseDBTLSRequired() (bool, error) {
	raw := strings.TrimSpace(os.Getenv("DB_TLS_REQUIRED"))
	switch raw {
	case "", "true":
		return true, nil
	case "false":
		return false, nil
	}
	vlog.Printf("db_tls.go", "parseDBTLSRequired", "decisão valor_invalido_recusado")
	return false, fmt.Errorf("%w: %q", ErrDBTLSRequired, raw)
}

// HostLoopback informa se host é loopback LITERAL: "localhost", um IP em
// 127.0.0.0/8 ou ::1 (com ou sem colchetes). Sem DNS: nome de container ou
// qualquer outro hostname não é loopback, mesmo que resolva para 127.x.
func HostLoopback(host string) bool {
	h := strings.TrimSpace(host)
	if strings.EqualFold(h, "localhost") {
		return true
	}
	h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
