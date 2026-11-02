package config

// TLS da conexão Go ↔ MariaDB (SEC-29).
//
// DB_TLS_CA (caminho de um PEM com a CA que assinou o certificado do servidor)
// liga o TLS: o PEM é carregado num x509.CertPool, registrado no driver com
// mysql.RegisterTLSConfig(DBTLSConfigName, ...) e o DSN ganha tls=custom.
// Sem DB_TLS_CA o DSN fica como antes (dev local sem TLS).
//
// DB_TLS_SERVER_NAME (opcional) é o nome conferido no certificado quando o
// host do DSN não está no SAN (ex.: DB_HOST=host.docker.internal e o
// certificado só cobre 192.168.168.106/localhost). Vazio = o próprio DB_HOST
// (o driver preenche o ServerName com o host do DSN).

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
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
	return fmt.Sprintf("ligado (ca=%s, server_name=%s, min=TLS1.2)", t.CAFile, nome)
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
		MinVersion: tls.VersionTLS12,
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
