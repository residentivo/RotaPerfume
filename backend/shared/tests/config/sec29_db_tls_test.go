package config_test

// SEC-29: TLS da conexão Go ↔ MariaDB. DB_TLS_CA (PEM da CA) liga o TLS:
// Load carrega a CA, registra a config "custom" no driver e o DSN ganha
// tls=custom. Sem DB_TLS_CA o DSN fica idêntico ao de antes. CA inexistente
// ou inválida e DB_TLS_SERVER_NAME inválido (ou sem CA) derrubam a subida.
// O handshake real (CA certa/errada, ServerName) está em tests/db.

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
)

const sec29SufixoDSN = "?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local&clientFoundRows=true"

// sec29Env: ambiente válido (credenciais incluídas) com DB_TLS_* definidos
// pelo caso (nil = ausente).
func sec29Env(t *testing.T, ca, serverName *string) {
	t.Helper()
	env := sec11Base()
	env["DB_USUARIO"] = sp("u")
	env["DB_SENHA"] = sp("p")
	env["DB_HOST"] = sp("h")
	env["DB_PORT"] = sp("3306")
	env["DB_NAME"] = sp("db")
	env["DB_TLS_CA"] = ca
	env["DB_TLS_SERVER_NAME"] = serverName
	sec11Env(t, env)
}

// sec29PEMDaCA grava um certificado de CA autoassinado (gerado na hora) num
// arquivo temporário e devolve o caminho.
func sec29PEMDaCA(t *testing.T) string {
	t.Helper()
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	modelo := &x509.Certificate{
		SerialNumber:          big.NewInt(29),
		Subject:               pkix.Name{CommonName: "SEC29_Teste_CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	require.NoError(t, err)
	return sec29Gravar(t, "ca.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func sec29Gravar(t *testing.T, nome string, dados []byte) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), nome)
	require.NoError(t, os.WriteFile(caminho, dados, 0o644))
	return caminho
}

func TestSEC29_SemDBTLSCA_DSNInalterado(t *testing.T) {
	sec29Env(t, nil, nil)
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.False(t, cfg.DBTLS.Ativo())
	assert.Equal(t, "u:p@tcp(h:3306)/db"+sec29SufixoDSN, cfg.DSN())
	assert.NotContains(t, cfg.DSN(), "tls=")
	assert.Contains(t, cfg.DBTLS.Descricao(), "desligado")

	// Só espaços conta como ausente.
	sec29Env(t, sp("   "), nil)
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.Equal(t, "u:p@tcp(h:3306)/db"+sec29SufixoDSN, cfg.DSN())
}

func TestSEC29_CAValida_DSNComTLSCustom(t *testing.T) {
	ca := sec29PEMDaCA(t)
	sec29Env(t, sp(ca), nil)
	cfg, err := config.Load()
	require.NoError(t, err)

	assert.True(t, cfg.DBTLS.Ativo())
	assert.Equal(t, ca, cfg.DBTLS.CAFile)
	assert.Empty(t, cfg.DBTLS.ServerName, "sem DB_TLS_SERVER_NAME o driver confere o DB_HOST")
	assert.Equal(t, "u:p@tcp(h:3306)/db"+sec29SufixoDSN+"&tls="+config.DBTLSConfigName, cfg.DSN())
	assert.Equal(t, "custom", config.DBTLSConfigName)
	assert.Contains(t, cfg.DBTLS.Descricao(), "ligado")
	assert.NotContains(t, cfg.DBTLS.Descricao(), "p@", "a descrição não leva credenciais")
}

func TestSEC29_ServerNameAplicado(t *testing.T) {
	ca := sec29PEMDaCA(t)
	for _, nome := range []string{"192.168.168.106", "localhost", "mariadb.lan"} {
		t.Run(nome, func(t *testing.T) {
			sec29Env(t, sp(ca), sp("  "+nome+" "))
			cfg, err := config.Load()
			require.NoError(t, err)
			assert.Equal(t, nome, cfg.DBTLS.ServerName)
			assert.Contains(t, cfg.DBTLS.Descricao(), "server_name="+nome)
			assert.True(t, strings.HasSuffix(cfg.DSN(), "&tls=custom"), cfg.DSN())
		})
	}
}

func TestSEC29_CAInexistenteOuInvalida_Erro(t *testing.T) {
	chave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	derChave, err := x509.MarshalECPrivateKey(chave)
	require.NoError(t, err)

	casos := []struct {
		nome, caminho, trecho string
	}{
		{"arquivo inexistente", filepath.Join(t.TempDir(), "nao-existe.pem"), "nao-existe.pem"},
		{"diretório", t.TempDir(), ""},
		{"arquivo vazio", sec29Gravar(t, "vazio.pem", nil), "nenhum certificado"},
		{"texto sem PEM", sec29Gravar(t, "lixo.pem", []byte("isto não é um certificado\n")), "nenhum certificado"},
		{"chave privada no lugar da CA", sec29Gravar(t, "chave.pem", pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: derChave})), "não é certificado"},
		{"bloco CERTIFICATE corrompido", sec29Gravar(t, "corrompido.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("lixo")})), "corrompido.pem"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			sec29Env(t, sp(c.caminho), nil)
			cfg, err := config.Load()
			require.ErrorIs(t, err, config.ErrDBTLSCA)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), "DB_TLS_CA")
			assert.Contains(t, err.Error(), c.trecho)
			assert.NotContains(t, err.Error(), "BEGIN", "o conteúdo do arquivo não vai para a mensagem")

			_, err = config.LoadDBTLS()
			assert.ErrorIs(t, err, config.ErrDBTLSCA)
		})
	}
}

func TestSEC29_ServerNameInvalidoOuSemCA_Erro(t *testing.T) {
	ca := sec29PEMDaCA(t)
	casos := []struct {
		nome       string
		ca, server *string
	}{
		{"server name sem CA", nil, sp("localhost")},
		{"com porta", sp(ca), sp("localhost:3306")},
		{"com esquema", sp(ca), sp("https://localhost")},
		{"curinga", sp(ca), sp("*.lan")},
		{"espaço interno", sp(ca), sp("maria db")},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			sec29Env(t, c.ca, c.server)
			cfg, err := config.Load()
			require.ErrorIs(t, err, config.ErrDBTLSServerName)
			assert.Nil(t, cfg)
		})
	}
}

// TestSEC29_CaminhoRelativo: relativo é resolvido a partir da raiz do
// repositório (backend/shared/go.mod), não do cwd do teste/comando; fora do
// repositório, a partir do cwd.
func TestSEC29_CaminhoRelativo(t *testing.T) {
	ca := sec29PEMDaCA(t)
	raiz, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)

	t.Run("dentro do repositório: relativo à raiz", func(t *testing.T) {
		rel, err := filepath.Rel(raiz, ca)
		require.NoError(t, err)
		sec29Env(t, sp(rel), nil)
		cfg, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, ca, cfg.DBTLS.CAFile)
	})

	t.Run("fora do repositório: relativo ao cwd", func(t *testing.T) {
		t.Chdir(filepath.Dir(ca))
		sec29Env(t, sp(filepath.Base(ca)), nil)
		cfg, err := config.Load()
		require.NoError(t, err)
		esperado, err := filepath.EvalSymlinks(ca)
		require.NoError(t, err)
		obtido, err := filepath.EvalSymlinks(cfg.DBTLS.CAFile)
		require.NoError(t, err)
		assert.Equal(t, esperado, obtido)
	})
}
