package deploy_test

// DEPLOY-08: deploy/validar-ca-banco.sh exige handshake TLS real. Sem
// handshake (erro 1130 do mariadbd antes do greeting, TLS desligado) o
// s_client também imprime "Verify return code: 0 (ok)"; o script não pode
// aceitar isso. E o estágio "CA do banco" do Jenkinsfile roda o container de
// validação com o IP da conta rotaperfumes_admin.

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Saída real do OpenSSL 3.0 diante do erro 1130 (servidor, 2026-11-04).
const saidaSClient1130 = `Only MySQL protocol version 10 is supported.
CONNECTED(00000003)
---
no peer certificate available
---
SSL handshake has read 78 bytes and written 0 bytes
Verification: OK
---
New, (NONE), Cipher is (NONE)
Verify return code: 0 (ok)
`

func TestDEPLOY08_ValidarCABanco_ExigeHandshake(t *testing.T) {
	precisaOpenSSL(t)
	ca, _ := gerarCA(t, t.TempDir(), "RotaPerfumes_MariaDB_CA")
	fp := "CA_FINGERPRINTS=" + fingerprint(t, ca) + "\n"
	alvo := []string{fp, "DB_ALVO=rotaperfumes-mariadb", "DB_HOST_PORT=3306", "TLS_NOME=rotaperfumes-mariadb"}

	casos := []struct {
		nome, saida, rc, erro string
	}{
		{"erro 1130 antes do greeting", saidaSClient1130, "1", "o container de validacao precisa de --ip 172.30.42.20"},
		{"Cipher NONE com Verify 0 e rc 0", "no peer certificate available\nNew, (NONE), Cipher is (NONE)\nVerify return code: 0 (ok)\n", "0", "sem handshake TLS"},
		{"sem linha Cipher", "Verify return code: 0 (ok)\n", "0", "sem handshake TLS"},
		{"handshake ok mas rc 1", "New, TLSv1.3, Cipher is TLS_AES_256_GCM_SHA384\nVerify return code: 0 (ok)\n", "1", "a CA NAO valida"},
		{"handshake ok sem Verify 0", "New, TLSv1.3, Cipher is TLS_AES_256_GCM_SHA384\nVerify return code: 62 (hostname mismatch)\n", "0", "a CA NAO valida"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			dir := t.TempDir()
			escrever(t, filepath.Join(dir, "sclient_saida"), c.saida, 0o644)
			escrever(t, filepath.Join(dir, "sclient_rc"), c.rc, 0o644)
			r := rodarValidarCA(t, dir, ca, alvo...)
			require.Error(t, r.err, r.stderr)
			assert.Contains(t, r.stderr, c.erro)
			assert.Empty(t, r.stdout, "sem sha256 no stdout")
		})
	}
	t.Run("handshake ok passa e mostra a cifra", func(t *testing.T) {
		r := rodarValidarCA(t, t.TempDir(), ca, alvo...)
		require.NoError(t, r.err, r.stderr)
		assert.Contains(t, r.stderr, "s_client: Cipher is TLS_AES_256_GCM_SHA384")
		assert.Regexp(t, `^[0-9a-f]{64}\n$`, r.stdout)
	})
}

func TestDEPLOY08_Jenkinsfile_ValidacaoCAComIPDaContaAdmin(t *testing.T) {
	jf := lerRaiz(t, "Jenkinsfile")
	assert.Contains(t, jf, "DB_CLIENTE_IP        = '172.30.42.20'")
	i := strings.Index(jf, "deploy/validar-ca-banco.sh)")
	require.GreaterOrEqual(t, i, 0)
	ini := strings.LastIndex(jf[:i], "SHA=$(docker run")
	require.GreaterOrEqual(t, ini, 0)
	run := jf[ini:i]
	assert.Regexp(t, regexp.MustCompile(`--network "\$DB_REDE" --ip "\$DB_CLIENTE_IP"`), run,
		"sem --ip o mariadbd manda o erro 1130 e não há handshake")
	assert.Contains(t, lerRaiz(t, "deploy", "validar-ca-banco.sh"), "--network rotaperfumes-db --ip 172.30.42.20")
}
