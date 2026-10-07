package services_test

// SEC-09: e-mail de alerta de reuso de refresh token.
//   - conteúdo (usuário x admin) contra o servidor SMTP falso de
//     email_smtp_test.go (fakeSMTP/smtpTLS/smtpCfg);
//   - saneamento de nome/IP/User-Agent (sem CR/LF/controles, sem cabeçalho
//     injetado) e truncamento do UA em 120 runes;
//   - ErrDestinatarioInvalido (vazio ou com CR/LF), também em
//     EnviarSenhaInicial;
//   - NoopEmailService: devolve nil e só loga "pulado", sem o destinatário.

import (
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/services"
)

const (
	sec09UserID  = int64(777)
	sec09TokenID = int64(4242)
	sec09Email   = "maria.silva@exemplo.com"
)

func sec09Alerta() services.AlertaReuso {
	return services.AlertaReuso{
		UsuarioID:    sec09UserID,
		EmailUsuario: sec09Email,
		IP:           "203.0.113.9",
		UserAgent:    "Mozilla/5.0 (X11)",
		TokenID:      sec09TokenID,
		// 15:04:05 UTC → 12:04:05 em -03:00.
		Quando: time.Date(2026, 9, 26, 15, 4, 5, 0, time.UTC),
	}
}

// sec09Enviar envia o alerta por um fakeSMTP novo e devolve (cabeçalhos,
// corpo, RCPT TO) como recebidos pelo servidor.
func sec09Enviar(t *testing.T, destinatario, nome string, a services.AlertaReuso, paraAdmin bool) (string, string, string) {
	t.Helper()
	cert, pool := smtpTLS(t)
	f := &fakeSMTP{cert: cert}
	host, port := f.start(t)
	svc, err := services.NewSMTPEmailServiceWithTLSConfig(smtpCfg(host, port), &tls.Config{RootCAs: pool})
	require.NoError(t, err)

	var _ services.AlertaSegurancaSender = svc
	require.NoError(t, svc.EnviarAlertaReusoToken(context.Background(), destinatario, nome, a, paraAdmin))

	f.mu.Lock()
	defer f.mu.Unlock()
	cab, corpo, ok := strings.Cut(f.data, "\r\n\r\n")
	require.True(t, ok, "mensagem sem separador cabeçalho/corpo: %q", f.data)
	return cab, corpo, f.rcptTo
}

func TestSEC09_EnviarAlertaReusoToken_ConteudoUsuarioEAdmin(t *testing.T) {
	casos := []struct {
		nome       string
		paraAdmin  bool
		dest       string
		wantCorpo  []string
		proibidos  []string
		proibidosI []string // case-insensitive
	}{
		{
			nome: "usuário: sem user_id/token_id/e-mail/token",
			dest: sec09Email,
			wantCorpo: []string{
				"Olá, Maria.\r\n",
				"Data/hora: 26/09/2026 12:04:05 (horário de Brasília)\r\n",
				"IP de origem: 203.0.113.9\r\n",
				"Navegador/dispositivo: Mozilla/5.0 (X11)\r\n",
				"encerramos todas as suas sessões",
				"troque a senha",
			},
			proibidos:  []string{"user_id", "token_id", "777", "4242", sec09Email, "hash"},
			proibidosI: []string{"token"},
		},
		{
			nome:      "admin: com user_id, e-mail do usuário e token_id",
			paraAdmin: true,
			dest:      "seg@rotaperfumes.com",
			wantCorpo: []string{
				"Alerta de segurança: reuso de sessão já encerrada",
				"Data/hora: 26/09/2026 12:04:05 (horário de Brasília)\r\n",
				"IP de origem: 203.0.113.9\r\n",
				"Navegador/dispositivo: Mozilla/5.0 (X11)\r\n",
				"Usuário: Maria (user_id=777, e-mail=" + sec09Email + ")\r\n",
				"token_id: 4242\r\n",
				"Todas as sessões do usuário foram encerradas",
			},
			proibidos: []string{"Olá,", "hash", "refresh_token"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cab, corpo, rcpt := sec09Enviar(t, c.dest, "Maria", sec09Alerta(), c.paraAdmin)

			assert.Equal(t, "<"+c.dest+">", rcpt)
			assert.Contains(t, cab, "To: "+c.dest+"\r\n")
			assert.Contains(t, cab, "Subject: Alerta de segurança — RotaPerfumes\r\n")
			for _, w := range c.wantCorpo {
				assert.Contains(t, corpo, w)
			}
			for _, p := range c.proibidos {
				assert.NotContains(t, corpo, p)
			}
			for _, p := range c.proibidosI {
				assert.NotContains(t, strings.ToLower(corpo), p)
			}
		})
	}
}

func TestSEC09_EnviarAlertaReusoToken_NomeVazioViraUsuario(t *testing.T) {
	casos := []struct{ nome, entrada string }{
		{"vazio", ""},
		{"só controles", "\r\n\t\x00"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, corpo, _ := sec09Enviar(t, sec09Email, c.entrada, sec09Alerta(), false)
			assert.Contains(t, corpo, "Olá, usuário.\r\n")
		})
	}
}

// Nome, IP e UA com CR/LF, NUL, TAB, DEL e U+2028/U+2029: nenhum cabeçalho
// injetado e nenhuma quebra de linha extra no corpo.
func TestSEC09_EnviarAlertaReusoToken_SaneiaControles(t *testing.T) {
	casos := []struct {
		nome      string
		paraAdmin bool
	}{
		{"usuário", false},
		{"admin", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			a := sec09Alerta()
			a.UserAgent = "Mozilla\r\nBcc: x@evil.com\r\n\r\nX-Injetado: 1"
			a.IP = "1.2.3.4\nSubject: falso"
			a.EmailUsuario = sec09Email + "\r\nCc: y@evil.com"
			nome := "Ma ria \x00\t\x7f\r\nReply-To: z@evil.com"

			cab, corpo, _ := sec09Enviar(t, "dest@exemplo.com", nome, a, c.paraAdmin)

			assert.NotContains(t, cab, "Bcc")
			assert.NotContains(t, cab, "X-Injetado")
			assert.NotContains(t, cab, "Reply-To")
			assert.NotContains(t, cab, "Cc:")
			assert.Equal(t, 1, strings.Count(cab, "Subject:"))
			for _, l := range strings.Split(corpo, "\r\n") {
				for _, pref := range []string{"Bcc:", "Cc:", "X-Injetado:", "Subject:", "Reply-To:"} {
					assert.False(t, strings.HasPrefix(l, pref), "linha injetada no corpo: %q", l)
				}
			}
			assert.Contains(t, corpo, "Navegador/dispositivo: MozillaBcc: x@evil.comX-Injetado: 1\r\n")
			assert.Contains(t, corpo, "IP de origem: 1.2.3.4Subject: falso\r\n")
			assert.NotContains(t, corpo, " ")
			assert.NotContains(t, corpo, " ")
			assert.NotContains(t, corpo, "\x00")
			assert.NotContains(t, corpo, "\t")
			assert.NotContains(t, corpo, "\x7f")
			// Nenhum LF solto (toda quebra é CRLF montada pelo próprio template).
			assert.Equal(t, strings.Count(corpo, "\n"), strings.Count(corpo, "\r\n"))
			if c.paraAdmin {
				assert.Contains(t, corpo, "Usuário: MariaReply-To: z@evil.com (user_id=777, e-mail="+sec09Email+"Cc: y@evil.com)\r\n")
			} else {
				assert.Contains(t, corpo, "Olá, MariaReply-To: z@evil.com.\r\n")
			}
		})
	}
}

func TestSEC09_EnviarAlertaReusoToken_TruncaUserAgent(t *testing.T) {
	casos := []struct {
		nome string
		ua   string
		want string
	}{
		{"curto não trunca", "curto", "curto"},
		{"exatamente 120 runes", strings.Repeat("a", 120), strings.Repeat("a", 120)},
		{"121 ASCII → 120", strings.Repeat("a", 121), strings.Repeat("a", 120)},
		{"multibyte 2 bytes", strings.Repeat("ç", 200), strings.Repeat("ç", 120)},
		{"multibyte 4 bytes (emoji)", strings.Repeat("\U0001F600", 130), strings.Repeat("\U0001F600", 120)},
		{"controles removidos antes de truncar", strings.Repeat("\r\n", 50) + strings.Repeat("é", 125), strings.Repeat("é", 120)},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			a := sec09Alerta()
			a.UserAgent = c.ua
			_, corpo, _ := sec09Enviar(t, sec09Email, "Maria", a, false)

			assert.True(t, utf8.ValidString(corpo), "corpo deve continuar UTF-8 válido")
			assert.Contains(t, corpo, "Navegador/dispositivo: "+c.want+"\r\n")
		})
	}
}

// Destinatário vazio ou com CR/LF é recusado antes de qualquer conexão (o
// SMTP configurado aponta para uma porta sem servidor: se discasse, o erro
// seria de dial, não ErrDestinatarioInvalido).
func TestSEC09_DestinatarioInvalido(t *testing.T) {
	svc, err := services.NewSMTPEmailService(smtpCfg("127.0.0.1", "1"))
	require.NoError(t, err)

	destinos := []struct{ nome, dest string }{
		{"vazio", ""},
		{"só espaços", "   "},
		{"CRLF + Bcc", "a@x.com\r\nBcc: evil@x.com"},
		{"LF", "a@x.com\n"},
		{"CR", "a@x.com\r"},
	}
	for _, d := range destinos {
		t.Run("alerta/"+d.nome, func(t *testing.T) {
			err := svc.EnviarAlertaReusoToken(context.Background(), d.dest, "Maria", sec09Alerta(), false)
			assert.ErrorIs(t, err, services.ErrDestinatarioInvalido)
		})
		t.Run("senha inicial/"+d.nome, func(t *testing.T) {
			err := svc.EnviarSenhaInicial(context.Background(), d.dest, "Maria", "S3nh@!")
			assert.ErrorIs(t, err, services.ErrDestinatarioInvalido)
		})
	}
}

// Destinatário válido segue para o dial (erro de conexão, não de validação).
func TestSEC09_DestinatarioValido_ChegaAoDial(t *testing.T) {
	svc, err := services.NewSMTPEmailService(smtpCfg("127.0.0.1", "1"))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = svc.EnviarAlertaReusoToken(ctx, " a@x.com ", "Maria", sec09Alerta(), true)
	require.Error(t, err)
	assert.NotErrorIs(t, err, services.ErrDestinatarioInvalido)
	assert.Contains(t, err.Error(), "smtp dial")
}

// Destinatário com espaços nas pontas chega aparado no RCPT TO e no To:.
func TestSEC09_DestinatarioComEspacos_ChegaAparado(t *testing.T) {
	for _, paraAdmin := range []bool{false, true} {
		cab, _, rcpt := sec09Enviar(t, " a@x.com ", "Maria", sec09Alerta(), paraAdmin)
		assert.Equal(t, "<a@x.com>", rcpt, "para_admin=%v", paraAdmin)
		assert.Contains(t, cab, "To: a@x.com\r\n", "para_admin=%v", paraAdmin)
		assert.NotContains(t, cab, "To:  a@x.com", "para_admin=%v", paraAdmin)
	}
}

func TestSEC09_Noop_EnviarAlertaReusoToken(t *testing.T) {
	casos := []struct {
		nome      string
		dest      string
		paraAdmin bool
		wantLog   string
	}{
		{"usuário", sec09Email, false, "para_admin=false"},
		{"admin", "seg@rotaperfumes.com", true, "para_admin=true"},
		{"destinatário vazio também não falha", "", false, "para_admin=false"},
	}
	svc := services.NewNoopEmailService()
	var _ services.AlertaSegurancaSender = svc

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var buf bytes.Buffer
			anterior := log.Writer()
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(anterior) })

			err := svc.EnviarAlertaReusoToken(context.Background(), c.dest, "Maria", sec09Alerta(), c.paraAdmin)
			// LOG-01: o skip é sinalizado ao chamador pelo erro sentinela.
			require.ErrorIs(t, err, services.ErrSMTPNaoConfigurado)

			out := buf.String()
			assert.Contains(t, out, "[email] alerta de seguranca pulado: SMTP não configurado user_id=777 "+c.wantLog)
			assert.NotContains(t, out, sec09Email, "log não pode conter o e-mail do usuário")
			assert.NotContains(t, out, "4242")
			assert.NotContains(t, out, "203.0.113.9")
		})
	}
}
