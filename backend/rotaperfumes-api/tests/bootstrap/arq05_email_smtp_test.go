package bootstrap_test

// ARQ-05 (cobertura do portão): com SMTP_USER/SMTP_PASSWORD/SMTP_FROM
// preenchidos, o Montar escolhe o EmailService SMTP (MAIL-01) e o log de
// subida leva o remetente mascarado, nunca a senha.

import (
	"bytes"
	"log"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
)

func TestARQ05_Montar_ComSMTPConfigurado(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	cfg := testCfg()
	cfg.SMTPHost, cfg.SMTPPort = "smtp.exemplo.test", "587"
	cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPFrom = "usuario", "senha-smtp-secreta", "remetente@exemplo.test"

	var buf bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(anterior)

	app := bootstrap.Montar(cfg, db)

	require.NotNil(t, app.Handler)
	saida := buf.String()
	assert.Contains(t, saida, "[server] EmailService SMTP configurado: host=smtp.exemplo.test port=587")
	assert.NotContains(t, saida, "senha-smtp-secreta")
	assert.NotContains(t, saida, "remetente@exemplo.test", "remetente só mascarado")
}
