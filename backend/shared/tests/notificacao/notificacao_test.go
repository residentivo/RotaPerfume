package notificacao_test

// ARQ-07: contexto Notificação. O envio SMTP (Strategy de TLS, AUTH só sobre
// TLS, CR/LF) é coberto pelas suítes MAIL01/SEC10_TLS/SEC09 em
// tests/services; o notificador (dedup, teto, fila, prazo), por
// rotaperfumes-api/tests/services/sec09_notifier_test.go. Aqui ficam o
// domínio, a montagem do notificador e a espera do shutdown (NB-6).

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/notificacao"
	"github.com/rotaperfumes/shared/notificacao/notificacaoapp"
)

func TestARQ07_NovoDestinatario(t *testing.T) {
	d, err := notificacao.NovoDestinatario("  ana@exemplo.com ")
	require.NoError(t, err)
	assert.Equal(t, "ana@exemplo.com", d.String())

	for _, ruim := range []string{"", "   ", "ana@exemplo.com\r\nBcc: x@y.z", "ana@exemplo.com\n", "\rana@exemplo.com"} {
		_, err := notificacao.NovoDestinatario(ruim)
		assert.ErrorIs(t, err, notificacao.ErrDestinatarioInvalido, "%q", ruim)
		assert.False(t, notificacao.DestinatarioValido(ruim), "%q", ruim)
	}
}

func TestARQ07_ModoTLS_String(t *testing.T) {
	assert.Equal(t, "implicit", notificacao.ModoTLSImplicito.String())
	assert.Equal(t, "starttls", notificacao.ModoSTARTTLS.String())
	_, err := notificacao.ResolverModoTLS("plain", "25")
	assert.ErrorIs(t, err, notificacao.ErrModoTLSInvalido, "não existe modo sem TLS (M-01)")
}

func TestARQ07_MensagemSenhaInicial(t *testing.T) {
	m := notificacao.MensagemSenhaInicial("Ana", "S3nh@Gerada")
	assert.Equal(t, "Sua senha de acesso — RotaPerfumes", m.Assunto)
	assert.True(t, strings.HasPrefix(m.Corpo, "Olá, Ana.\r\n"))
	assert.Contains(t, m.Corpo, "\r\n    S3nh@Gerada\r\n")
}

func TestARQ07_MensagemAlertaReuso_SeparadoresUnicode(t *testing.T) {
	a := notificacao.AlertaReuso{UsuarioID: 7, IP: "1.2.3.4 x", UserAgent: "UA y", TokenID: 9, Quando: time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)}
	m := notificacao.MensagemAlertaReuso("Ana ", a, false)
	assert.NotContains(t, m.Corpo, " ")
	assert.NotContains(t, m.Corpo, " ")
	assert.Contains(t, m.Corpo, "IP de origem: 1.2.3.4x\r\n")
	assert.NotContains(t, m.Corpo, "token_id", "ao usuário, sem identificadores internos")
}

type contatosFixos struct{}

func (contatosFixos) ContatoDoUsuario(context.Context, int64) (notificacao.Contato, error) {
	return notificacao.Contato{Nome: "Ana", Email: "ana@exemplo.com"}, nil
}

// enviadorBloqueado segura cada envio até liberar.
type enviadorBloqueado struct {
	iniciou chan struct{}
	liberar chan struct{}
}

func (e enviadorBloqueado) EnviarAlertaReusoToken(context.Context, string, string, notificacao.AlertaReuso, bool) error {
	e.iniciou <- struct{}{}
	<-e.liberar
	return nil
}

func TestARQ07_Notifier_ExigeDependencias(t *testing.T) {
	completo := notificacaoapp.Deps{Contatos: contatosFixos{}, Enviador: enviadorBloqueado{}, Relogio: time.Now, Executor: notificacaoapp.EmGoroutine}
	assert.NotPanics(t, func() { notificacaoapp.NovoAlertaSegurancaNotifier(completo) })
	faltando := []func(*notificacaoapp.Deps){
		func(d *notificacaoapp.Deps) { d.Contatos = nil },
		func(d *notificacaoapp.Deps) { d.Enviador = nil },
		func(d *notificacaoapp.Deps) { d.Relogio = nil },
		func(d *notificacaoapp.Deps) { d.Executor = nil },
	}
	for i, tirar := range faltando {
		d := completo
		tirar(&d)
		assert.Panics(t, func() { notificacaoapp.NovoAlertaSegurancaNotifier(d) }, "dependência %d", i)
	}
}

// NB-6: o shutdown espera os alertas em andamento antes do pool.Close.
func TestARQ07_NB6_AguardarEsperaOsEnviosEmAndamento(t *testing.T) {
	e := enviadorBloqueado{iniciou: make(chan struct{}, 2), liberar: make(chan struct{})}
	n := notificacaoapp.NovoAlertaSegurancaNotifier(notificacaoapp.Deps{
		Contatos: contatosFixos{}, Enviador: e, Relogio: time.Now, Executor: notificacaoapp.EmGoroutine,
	})
	// ARQ-07 NB-B: Aguardar fecha o notificador; o caso "sem envios" fica em
	// TestARQ07_NBB_AguardarSemEnviosVoltaNaHora.
	n.Notificar(5, 1, "1.2.3.4", "UA")
	<-e.iniciou // o envio está em andamento (bloqueado)
	assert.False(t, n.Aguardar(20*time.Millisecond), "não pode dar o envio por encerrado")

	var wg sync.WaitGroup
	wg.Add(1)
	var terminou bool
	go func() {
		defer wg.Done()
		terminou = n.Aguardar(5 * time.Second)
	}()
	close(e.liberar)
	wg.Wait()
	assert.True(t, terminou)
}

func TestARQ07_EmGoroutine(t *testing.T) {
	feito := make(chan struct{})
	notificacaoapp.EmGoroutine(func() { close(feito) })
	select {
	case <-feito:
	case <-time.After(5 * time.Second):
		t.Fatal(errors.New("EmGoroutine não executou a função"))
	}
}

// capturarLog redireciona o log padrão para um buffer até o fim do teste.
func capturarLog(t *testing.T) *logSeguro {
	t.Helper()
	buf := &logSeguro{}
	anterior := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return buf
}

// logSeguro é um buffer de log seguro para escrita concorrente.
type logSeguro struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logSeguro) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logSeguro) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// enviadorEcoando falha ecoando o destino em minúsculas, como um 550 real.
type enviadorEcoando struct {
	mu       sync.Mutex
	destinos []string
}

func (e *enviadorEcoando) EnviarAlertaReusoToken(_ context.Context, destino, _ string, _ notificacao.AlertaReuso, _ bool) error {
	e.mu.Lock()
	e.destinos = append(e.destinos, destino)
	e.mu.Unlock()
	return errors.New("550 <" + strings.ToLower(destino) + ">: mailbox unavailable")
}

func (e *enviadorEcoando) total() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.destinos)
}

func executarJa(f func()) { f() }

// ARQ-07 NB-B (a): a falha do envio ao admin não vaza o endereço do admin
// (nem o do usuário) no log.
func TestARQ07_NBB_FalhaAoAdminNaoVazaEndereco(t *testing.T) {
	logs := capturarLog(t)
	e := &enviadorEcoando{}
	n := notificacaoapp.NovoAlertaSegurancaNotifier(notificacaoapp.Deps{
		Contatos: contatosFixos{}, Enviador: e, Admins: []string{"Admin.Chefe@Empresa.com"},
		Relogio: time.Now, Executor: executarJa,
	})

	n.Notificar(5, 1, "1.2.3.4", "UA")

	assert.Equal(t, 2, e.total(), "usuário e admin")
	saida := logs.String()
	assert.Contains(t, saida, "falha ao enviar alerta: user_id=5: 550 <[email-usuario]>")
	assert.NotContains(t, strings.ToLower(saida), "admin.chefe@empresa.com")
	assert.NotContains(t, saida, "ana@exemplo.com")
}

// ARQ-07 NB-B (c): sem envio em andamento, Aguardar volta na hora mesmo com
// prazo zero.
func TestARQ07_NBB_AguardarSemEnviosVoltaNaHora(t *testing.T) {
	n := notificacaoapp.NovoAlertaSegurancaNotifier(notificacaoapp.Deps{
		Contatos: contatosFixos{}, Enviador: &enviadorEcoando{}, Relogio: time.Now, Executor: executarJa,
	})
	assert.True(t, n.Aguardar(0))
}

// ARQ-07 NB-B (b): depois de Aguardar (shutdown), Notificar não envia, não
// entra em pânico (WaitGroup reutilizado) e loga a supressão.
func TestARQ07_NBB_NotificarAposAguardarNaoEnvia(t *testing.T) {
	logs := capturarLog(t)
	e := &enviadorEcoando{}
	n := notificacaoapp.NovoAlertaSegurancaNotifier(notificacaoapp.Deps{
		Contatos: contatosFixos{}, Enviador: e, Admins: []string{"admin@empresa.com"},
		Relogio: time.Now, Executor: notificacaoapp.EmGoroutine,
	})
	require.True(t, n.Aguardar(time.Second))

	assert.NotPanics(t, func() {
		for i := int64(1); i <= notificacaoapp.AlertaMaxEnviosSimultaneos+1; i++ {
			n.Notificar(i, i, "1.2.3.4", "UA")
		}
	})

	assert.True(t, n.Aguardar(time.Second))
	assert.Equal(t, 0, e.total(), "nenhum envio após o fechamento")
	saida := logs.String()
	assert.Contains(t, saida, "[auth][seguranca] alerta suprimido (encerrando): user_id=1")
	assert.Contains(t, saida, "alerta suprimido (encerrando): user_id=5",
		"a vaga da fila é devolvida: o 5º não cai em fila cheia")
	assert.NotContains(t, saida, "fila cheia")
}
