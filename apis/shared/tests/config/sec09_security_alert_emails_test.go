package config_test

// SEC-09: SECURITY_ALERT_EMAILS — destinatários administrativos dos alertas
// de segurança. Separados por vírgula, com trim; vazios, sem "@" ou com
// CR/LF são descartados.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

func TestSEC09_ParseSecurityAlertEmails(t *testing.T) {
	casos := []struct {
		nome string
		raw  string
		want []string
	}{
		{"vazio", "", nil},
		{"só espaços e vírgulas", " , ,, ", nil},
		{"especificação do SecBrain", " a@x.com, ,b@y.com,c\r\n@z", []string{"a@x.com", "b@y.com"}},
		{"um só", "admin@rota.com", []string{"admin@rota.com"}},
		{"sem @ descartado", "semarroba, ok@x.com", []string{"ok@x.com"}},
		{"só LF descartado", "a\n@x.com,b@y.com", []string{"b@y.com"}},
		{"só CR descartado", "a\r@x.com,b@y.com", []string{"b@y.com"}},
		{"CR/LF com cabeçalho injetado", "a@x.com\r\nBcc: evil@x.com", nil},
		{"trim de tab e espaço", "\ta@x.com  ,  b@y.com\t", []string{"a@x.com", "b@y.com"}},
		{"mantém ordem e duplicados", "b@y.com,a@x.com,b@y.com", []string{"b@y.com", "a@x.com", "b@y.com"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			got := config.ParseSecurityAlertEmails(c.raw)
			if c.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, c.want, got)
		})
	}
}

func TestSEC09_Load_SecurityAlertEmails(t *testing.T) {
	casos := []struct {
		nome string
		raw  string
		want []string
	}{
		{"ausente/vazio → vazio", "", nil},
		{"lista com lixo", " a@x.com, ,b@y.com,c\r\n@z", []string{"a@x.com", "b@y.com"}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "segredo")
			t.Setenv("JWT_TTL", "24h")
			t.Setenv("BCRYPT_COST", "12")
			t.Setenv("SECURITY_ALERT_EMAILS", c.raw)
			cfg, err := config.Load()
			require.NoError(t, err)
			if c.want == nil {
				assert.Empty(t, cfg.SecurityAlertEmails)
				return
			}
			assert.Equal(t, c.want, cfg.SecurityAlertEmails)
		})
	}
}
