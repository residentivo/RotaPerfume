package db_test

// TST-12: o cliente Go do banco exige TLS 1.3 (MinVersion). Handshake real
// contra o servidor MySQL falso do SEC-29 (127.0.0.1, sem banco real):
//   - servidor 1.2+1.3 (como o MariaDB de produção) → conecta em TLS 1.3;
//   - servidor só TLS 1.2 → recusado;
//   - servidor só TLS 1.1 → recusado;
//   - certificado do servidor vencido (CA certa, nome certo) → recusado.
// Em todos os casos de recusa, nenhum login acontece e a senha não aparece no
// erro.

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/db"
)

// certServidorValidade emite o certificado do servidor (SAN mariadb.sec29)
// com a janela de validade pedida.
func (ca sec29CA) certServidorValidade(t *testing.T, inicio, fim time.Time) tls.Certificate {
	t.Helper()
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	modelo := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "MariaDB Server"},
		DNSNames:     []string{sec29NomeServidor},
		NotBefore:    inicio,
		NotAfter:     fim,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, ca.cert, &chave.PublicKey, ca.chave)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: chave}
}

// tst12Servidor sobe o servidor MySQL falso com a config TLS do caso e
// registra a versão negociada em cada handshake concluído.
func tst12Servidor(t *testing.T, cfg *tls.Config) (addr string, logins *atomic.Int32, versao *atomic.Uint32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	logins, versao = &atomic.Int32{}, &atomic.Uint32{}
	cfg = cfg.Clone()
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		versao.Store(uint32(cs.Version))
		return nil
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go atenderMySQLTLS(c, cfg, logins)
		}
	}()
	return ln.Addr().String(), logins, versao
}

func TestTST12_TLS_Servidor12e13_NegociaTLS13(t *testing.T) {
	ca := sec29NovaCA(t, "TST12_CA")
	addr, logins, versao := tst12Servidor(t, &tls.Config{
		Certificates: []tls.Certificate{ca.certServidor(t)},
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS13,
	})
	conn, err := db.Open(sec29DSN(t, addr, ca.pem, sec29NomeServidor))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	assert.EqualValues(t, 1, logins.Load())
	assert.EqualValues(t, tls.VersionTLS13, versao.Load(), "o cliente negocia TLS 1.3")
}

func TestTST12_TLS_Recusas(t *testing.T) {
	ca := sec29NovaCA(t, "TST12_CA")
	agora := time.Now()
	casos := []struct {
		nome     string
		servidor *tls.Config
		trecho   string
	}{
		{
			nome: "servidor só TLS 1.2",
			servidor: &tls.Config{Certificates: []tls.Certificate{ca.certServidor(t)},
				MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12},
			trecho: "protocol version",
		},
		{
			nome: "servidor só TLS 1.1",
			servidor: &tls.Config{Certificates: []tls.Certificate{ca.certServidor(t)},
				MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11},
			trecho: "protocol version",
		},
		{
			nome: "certificado do servidor vencido",
			servidor: &tls.Config{Certificates: []tls.Certificate{ca.certServidorValidade(t, agora.Add(-48*time.Hour), agora.Add(-24*time.Hour))},
				MinVersion: tls.VersionTLS12},
			trecho: "expired",
		},
		{
			nome: "certificado do servidor ainda não válido",
			servidor: &tls.Config{Certificates: []tls.Certificate{ca.certServidorValidade(t, agora.Add(24*time.Hour), agora.Add(48*time.Hour))},
				MinVersion: tls.VersionTLS12},
			trecho: "x509",
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			addr, logins, _ := tst12Servidor(t, c.servidor)
			conn, err := db.Open(sec29DSN(t, addr, ca.pem, sec29NomeServidor))
			require.Error(t, err)
			assert.Nil(t, conn)
			assert.Contains(t, err.Error(), c.trecho, "erro claro: %v", err)
			assert.NotContains(t, err.Error(), "segredo-sec29")
			assert.EqualValues(t, 0, logins.Load(), "nenhum login fora do TLS 1.3 válido")
		})
	}
}
