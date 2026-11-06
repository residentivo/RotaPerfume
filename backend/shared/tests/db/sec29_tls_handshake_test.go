package db_test

// SEC-29: handshake TLS real entre o driver e um servidor mínimo do protocolo do MariaDB em
// 127.0.0.1 (SSLRequest → TLS → login → OK). O certificado do servidor é
// assinado por uma CA gerada no teste e só tem SAN DNS "mariadb.sec29".
//   - CA certa + DB_TLS_SERVER_NAME=mariadb.sec29 → conecta, log com tls=ligado;
//   - CA certa sem ServerName (host 127.0.0.1 fora do SAN) → falha;
//   - CA errada → falha;
//   - servidor sem TLS com tls=custom → falha (sem rebaixar para texto claro).
// maskDSN continua sem vazar a senha com tls=custom no DSN.

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
)

const sec29NomeServidor = "mariadb.sec29"

type sec29CA struct {
	cert  *x509.Certificate
	chave *ecdsa.PrivateKey
	pem   string // caminho do PEM
}

func sec29NovaCA(t *testing.T, cn string) sec29CA {
	t.Helper()
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	modelo := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	caminho := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caminho, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644))
	return sec29CA{cert: cert, chave: chave, pem: caminho}
}

// certServidor emite o certificado do servidor (SAN só DNS:mariadb.sec29).
func (ca sec29CA) certServidor(t *testing.T) tls.Certificate {
	t.Helper()
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	modelo := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "MariaDB Server"},
		DNSNames:     []string{sec29NomeServidor},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, ca.cert, &chave.PublicKey, ca.chave)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: chave}
}

// fakeMySQLTLS é o fakeMySQL com CLIENT_SSL: depois do SSLRequest do
// cliente, sobe TLS com cert e segue o protocolo dentro do túnel. logins
// conta os logins concluídos sobre TLS.
func fakeMySQLTLS(t *testing.T, cert tls.Certificate) (addr string, logins *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	logins = &atomic.Int32{}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go atenderMySQLTLS(c, cfg, logins)
		}
	}()
	return ln.Addr().String(), logins
}

func atenderMySQLTLS(c net.Conn, cfg *tls.Config, logins *atomic.Int32) {
	defer c.Close()
	const (
		clientLongPassword = 0x00000001
		clientProtocol41   = 0x00000200
		clientSSL          = 0x00000800
		clientTransactions = 0x00002000
		clientSecureConn   = 0x00008000
		clientPluginAuth   = 0x00080000
	)
	caps := uint32(clientLongPassword | clientProtocol41 | clientSSL | clientTransactions | clientSecureConn | clientPluginAuth)
	var hs bytes.Buffer
	hs.WriteByte(10)
	hs.WriteString("11.8.8-MariaDB-fake\x00")
	hs.Write([]byte{1, 0, 0, 0})
	hs.WriteString("abcdefgh")
	hs.WriteByte(0)
	_ = binary.Write(&hs, binary.LittleEndian, uint16(caps))
	hs.WriteByte(45)
	hs.Write([]byte{0x02, 0x00})
	_ = binary.Write(&hs, binary.LittleEndian, uint16(caps>>16))
	hs.WriteByte(21)
	hs.Write(make([]byte, 10))
	hs.WriteString("ijklmnopqrst\x00")
	hs.WriteString("mysql_native_password\x00")
	if escreverPacote(c, 0, hs.Bytes()) != nil {
		return
	}
	// SSLRequest (32 bytes, sem usuário) → handshake TLS.
	if _, p, err := lerPacote(c); err != nil || len(p) != 32 {
		return
	}
	tc := tls.Server(c, cfg)
	if tc.Handshake() != nil {
		return
	}
	seq, _, err := lerPacote(tc)
	if err != nil || escreverPacote(tc, seq+1, pacoteOK) != nil {
		return
	}
	logins.Add(1)
	for {
		seq, p, err := lerPacote(tc)
		if err != nil || len(p) == 0 || p[0] == 0x01 {
			return
		}
		if escreverPacote(tc, seq+1, pacoteOK) != nil {
			return
		}
	}
}

// sec29DSN monta o DSN pelo caminho de produção (LoadDBTLS + Config.DSN).
func sec29DSN(t *testing.T, addr, caPEM, serverName string) string {
	t.Helper()
	t.Setenv("DB_TLS_CA", caPEM)
	t.Setenv("DB_TLS_SERVER_NAME", serverName)
	dbTLS, err := config.LoadDBTLS()
	require.NoError(t, err)
	host, porta, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	cfg := config.Config{DBUsuario: "user", DBSenha: "segredo-sec29", DBHost: host, DBPort: porta, DBName: "rota", DBTLS: dbTLS}
	return cfg.DSN()
}

func TestSEC29_TLS_CACertaEServerName_Conecta(t *testing.T) {
	ca := sec29NovaCA(t, "SEC29_CA")
	addr, logins := fakeMySQLTLS(t, ca.certServidor(t))
	dsn := sec29DSN(t, addr, ca.pem, sec29NomeServidor)
	require.True(t, strings.HasSuffix(dsn, "&tls=custom"), dsn)

	linha := abrirELogar(t, dsn)
	assert.EqualValues(t, 1, logins.Load(), "o login aconteceu dentro do TLS")
	assert.Equal(t, "[db] conexão MariaDB estabelecida (host=user:****@tcp("+addr+")/rota"+
		"?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local&clientFoundRows=true&time_zone=%27-03%3A00%27&tls=custom, tls=ligado (custom))", linha)
	assert.NotContains(t, linha, "segredo-sec29", "a senha nunca vai para o log")
}

func TestSEC29_TLS_FalhasDeVerificacao(t *testing.T) {
	ca := sec29NovaCA(t, "SEC29_CA")
	outra := sec29NovaCA(t, "SEC29_CA_Errada")
	addr, logins := fakeMySQLTLS(t, ca.certServidor(t))

	casos := []struct {
		nome, caPEM, serverName, trecho string
	}{
		{"CA certa, host fora do SAN e sem DB_TLS_SERVER_NAME", ca.pem, "", "x509"},
		{"CA certa, DB_TLS_SERVER_NAME que não está no SAN", ca.pem, "localhost", "x509"},
		{"CA errada", outra.pem, sec29NomeServidor, "x509"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			conn, err := db.Open(sec29DSN(t, addr, c.caPEM, c.serverName))
			require.Error(t, err)
			assert.Nil(t, conn)
			assert.Contains(t, err.Error(), c.trecho)
			assert.NotContains(t, err.Error(), "segredo-sec29")
		})
	}
	assert.EqualValues(t, 0, logins.Load(), "nenhum login com certificado rejeitado")
}

func TestSEC29_TLS_ServidorSemTLS_Falha(t *testing.T) {
	ca := sec29NovaCA(t, "SEC29_CA")
	addr := fakeMySQL(t) // não anuncia CLIENT_SSL

	var buf bytes.Buffer
	saida := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saida) })

	conn, err := db.Open(sec29DSN(t, addr, ca.pem, sec29NomeServidor))
	require.Error(t, err, "tls=custom nunca cai para texto claro")
	assert.Nil(t, conn)
	assert.NotContains(t, buf.String(), "estabelecida")
}
