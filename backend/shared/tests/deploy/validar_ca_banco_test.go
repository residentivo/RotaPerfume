package deploy_test

// SEC-40 / SEC-45 (Lote 30, TST-15): deploy/validar-ca-banco.sh com
// CA_FINGERPRINTS obrigatória e DB_ALVO. Roda o script real com bash e
// openssl locais; o "timeout" (que envolve o openssl s_client) é um stub que
// registra os argumentos, então nada conecta em banco.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const stubTimeoutCA = `#!/bin/bash
printf '%s\n' "$*" >>"$FAKE/sclient"
[[ -f "$FAKE/sclient_falha" ]] && { echo "verify error:num=62:hostname mismatch"; exit 1; }
echo "Verify return code: 0 (ok)"
`

type execCA struct {
	stdout, stderr string
	err            error
	sclient        string
}

func rodarValidarCA(t *testing.T, dir, ca string, env ...string) execCA {
	t.Helper()
	precisaBash(t)
	precisaOpenSSL(t)
	escrever(t, filepath.Join(dir, "bin", "timeout"), stubTimeoutCA, 0o755)
	cmd := exec.Command("bash", caminhoRaiz(t, "deploy", "validar-ca-banco.sh"))
	cmd.Env = append(ambienteLimpo(), "FAKE="+dir, "PATH="+filepath.Join(dir, "bin")+":"+os.Getenv("PATH"), "CA="+ca)
	cmd.Env = append(cmd.Env, env...)
	var so, se strings.Builder
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	b, _ := os.ReadFile(filepath.Join(dir, "sclient"))
	return execCA{stdout: so.String(), stderr: se.String(), err: err, sclient: string(b)}
}

func sha256Arquivo(t *testing.T, p string) string {
	t.Helper()
	s := sha256.Sum256([]byte(mustRead(t, p)))
	return hex.EncodeToString(s[:])
}

func TestSEC40_ValidarCABanco_Fingerprints(t *testing.T) {
	precisaOpenSSL(t)
	base := t.TempDir()
	ca1, key1 := gerarCA(t, filepath.Join(base, "ca1"), "RotaPerfumes_MariaDB_CA")
	ca2, _ := gerarCA(t, filepath.Join(base, "ca2"), "RotaPerfumes_MariaDB_CA_nova")
	fp1, fp2 := fingerprint(t, ca1), fingerprint(t, ca2)
	bundle := filepath.Join(base, "bundle.pem")
	escrever(t, bundle, mustRead(t, ca1)+mustRead(t, ca2), 0o644)
	comChave := filepath.Join(base, "com-chave.pem")
	escrever(t, comChave, mustRead(t, ca1)+mustRead(t, key1), 0o644)
	crlf := filepath.Join(base, "crlf.pem")
	escrever(t, crlf, strings.ReplaceAll(mustRead(t, ca1), "\n", "\r\n"), 0o644)
	alvo := []string{"DB_ALVO=rotaperfumes-mariadb", "DB_HOST_PORT=3306", "TLS_NOME=rotaperfumes-mariadb"}

	casos := []struct {
		nome    string
		ca      string
		fps     string
		env     []string
		erro    string
		stderr  []string
		sclient bool
	}{
		{"certo", ca1, fp1 + "\n", alvo, "", []string{"fingerprint " + fp1 + ": na lista", "CA do banco OK (1 certificado(s))"}, true},
		{"certo com comentario, CRLF e espacos", ca1, "# CA atual\r\n\r\n  " + fp1 + "  \r\n", alvo, "", []string{"na lista"}, true},
		{"certo na rotacao (dois aceitos)", ca1, fp2 + "\n" + fp1 + "\n", alvo, "", []string{"na lista"}, true},
		{"errado", ca1, fp2 + "\n", alvo, "1 de 1 certificado(s) do arquivo do host fora de deploy/db-ca.fingerprint", []string{"fingerprint " + fp1 + ": NAO esta em deploy/db-ca.fingerprint"}, false},
		{"vazio", ca1, "", alvo, "CA_FINGERPRINTS vazio", nil, false},
		{"so comentarios", ca1, "# nada\n\n", alvo, "CA_FINGERPRINTS vazio", nil, false},
		{"maiusculo", ca1, strings.ToUpper(fp1), alvo, "CA_FINGERPRINTS com linha invalida", nil, false},
		{"com dois-pontos", ca1, fp1[:2] + ":" + fp1[2:], alvo, "CA_FINGERPRINTS com linha invalida", nil, false},
		{"bundle com cert fora da lista", bundle, fp1 + "\n", alvo, "1 de 2 certificado(s) do arquivo do host fora", []string{fp1 + ": na lista", fp2 + ": NAO esta"}, false},
		{"bundle todo na lista", bundle, fp1 + "\n" + fp2 + "\n", alvo, "", []string{"CA do banco OK (2 certificado(s))"}, true},
		{"tres aceitos avisa", ca1, fp1 + "\n" + fp2 + "\n" + strings.Repeat("a", 64) + "\n", alvo, "", []string{"AVISO: 3 fingerprints aceitos"}, true},
		{"CRLF no PEM", crlf, fp1, alvo, "", []string{"na lista"}, true},
		{"chave privada no arquivo", comChave, fp1, alvo, "bloco PEM que nao e certificado", nil, false},
		{"arquivo inexistente", filepath.Join(base, "nao-existe.pem"), fp1, alvo, "arquivo da CA nao legivel", nil, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := rodarValidarCA(t, t.TempDir(), c.ca, append([]string{"CA_FINGERPRINTS=" + c.fps}, c.env...)...)
			for _, s := range c.stderr {
				assert.Contains(t, r.stderr, s)
			}
			assert.Equal(t, c.sclient, r.sclient != "", "s_client só depois de todos os fingerprints conferirem: %s", r.stderr)
			if c.erro != "" {
				require.Error(t, r.err, r.stderr)
				assert.Contains(t, r.stderr, c.erro)
				assert.Empty(t, r.stdout, "sem sha256 no stdout quando falha")
				return
			}
			require.NoError(t, r.err, r.stderr)
			assert.Equal(t, sha256Arquivo(t, c.ca)+"\n", r.stdout, "stdout só com o sha256 do arquivo (label do compose)")
		})
	}
}

func TestSEC45_ValidarCABanco_AlvoENome(t *testing.T) {
	precisaOpenSSL(t)
	base := t.TempDir()
	ca, _ := gerarCA(t, base, "RotaPerfumes_MariaDB_CA")
	fp := "CA_FINGERPRINTS=" + fingerprint(t, ca)
	casos := []struct {
		nome    string
		env     []string
		erro    string
		sclient []string
	}{
		{"rede interna por nome", []string{"DB_ALVO=rotaperfumes-mariadb", "DB_HOST_PORT=3306", "TLS_NOME=rotaperfumes-mariadb"}, "",
			[]string{"-starttls mysql -connect rotaperfumes-mariadb:3306", "-verify_return_error", "-verify_hostname rotaperfumes-mariadb -servername rotaperfumes-mariadb"}},
		{"IP no TLS_NOME usa -verify_ip", []string{"DB_ALVO=192.168.168.106", "DB_HOST_PORT=3306", "TLS_NOME=192.168.168.106"}, "",
			[]string{"-connect 192.168.168.106:3306", "-verify_ip 192.168.168.106"}},
		{"SITE_IP legado sem DB_ALVO", []string{"SITE_IP=10.0.0.5", "DB_HOST_PORT=3307", "TLS_NOME=rotaperfumes-mariadb"}, "",
			[]string{"-connect 10.0.0.5:3307"}},
		{"sem alvo", []string{"DB_HOST_PORT=3306", "TLS_NOME=x"}, "DB_ALVO (ou SITE_IP) obrigatorio", nil},
		{"DB_ALVO invalido", []string{"DB_ALVO=a;b", "DB_HOST_PORT=3306", "TLS_NOME=x"}, "DB_ALVO invalido", nil},
		{"DB_ALVO 0.0.0.0", []string{"DB_ALVO=0.0.0.0", "DB_HOST_PORT=3306", "TLS_NOME=x"}, "DB_ALVO invalido", nil},
		{"TLS_NOME invalido", []string{"DB_ALVO=h", "DB_HOST_PORT=3306", "TLS_NOME=h:1"}, "DB_TLS_SERVER_NAME invalido", nil},
		{"porta invalida", []string{"DB_ALVO=h", "DB_HOST_PORT=33a", "TLS_NOME=h"}, "DB_HOST_PORT invalida", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := rodarValidarCA(t, t.TempDir(), ca, append([]string{fp}, c.env...)...)
			if c.erro != "" {
				require.Error(t, r.err, r.stderr)
				assert.Contains(t, r.stderr, c.erro)
				assert.Empty(t, r.sclient)
				return
			}
			require.NoError(t, r.err, r.stderr)
			for _, s := range c.sclient {
				assert.Contains(t, r.sclient, s)
			}
			assert.Contains(t, r.stderr, "s_client: Verify return code: 0 (ok)")
		})
	}
	t.Run("s_client recusa", func(t *testing.T) {
		dir := t.TempDir()
		escrever(t, filepath.Join(dir, "sclient_falha"), "", 0o644)
		r := rodarValidarCA(t, dir, ca, fp, "DB_ALVO=rotaperfumes-mariadb", "DB_HOST_PORT=3306", "TLS_NOME=rotaperfumes-mariadb")
		require.Error(t, r.err)
		assert.Contains(t, r.stderr, "a CA NAO valida o certificado servido em rotaperfumes-mariadb:3306")
		assert.Contains(t, r.stderr, "verify error")
		assert.Empty(t, r.stdout)
	})
}

// O fingerprint versionado aceita a CA de deploy/ca-cert.pem no script real.
func TestSEC40_ValidarCABanco_FingerprintDoRepositorio(t *testing.T) {
	ca := caminhoRaiz(t, "deploy", "ca-cert.pem")
	if _, err := os.Stat(ca); err != nil {
		t.Skip("deploy/ca-cert.pem ausente nesta máquina")
	}
	r := rodarValidarCA(t, t.TempDir(), ca, "CA_FINGERPRINTS="+lerRaiz(t, "deploy", "db-ca.fingerprint"),
		"DB_ALVO=rotaperfumes-mariadb", "DB_HOST_PORT=3306", "TLS_NOME=rotaperfumes-mariadb")
	require.NoError(t, r.err, r.stderr)
	assert.Contains(t, r.stderr, ": na lista")
}
