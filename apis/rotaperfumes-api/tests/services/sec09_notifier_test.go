package services_test

// SEC-09: AlertaSegurancaNotifier — alerta de reuso de refresh token ao
// usuário e aos admins, com dedup por usuário (30 min), teto global (20/h,
// janela fixa), no máximo 4 envios simultâneos (fila cheia → descarta),
// contexto próprio de 30 s e log sem o e-mail do usuário.
//
// Fakes: sender (grava chamadas, pode falhar por destinatário ou bloquear
// em canais), relógio manual, lookup de usuário e executor (síncrono ou
// goroutine que sinaliza o fim). Nenhum sleep: a sincronização é por canais.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

const (
	sec09Email   = "maria.silva@exemplo.com"
	sec09Admin1  = "seg1@rotaperfumes.com"
	sec09Admin2  = "seg2@rotaperfumes.com"
	sec09IP      = "203.0.113.9"
	sec09UA      = "Mozilla/5.0 (X11)"
	sec09TokenID = int64(4242)
)

var sec09Admins = []string{sec09Admin1, sec09Admin2}

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type sec09Chamada struct {
	dest      string
	nome      string
	alerta    sharedsvc.AlertaReuso
	paraAdmin bool
	deadline  time.Time
	temPrazo  bool
	ctxErr    error
	chamadaEm time.Time
}

type sec09Sender struct {
	mu      sync.Mutex
	chamdas []sec09Chamada
	errPor  map[string]error // destinatário → erro

	// bloqueio opcional: sinaliza em iniciou e espera liberar.
	iniciou chan string
	liberar chan struct{}
}

func (s *sec09Sender) EnviarAlertaReusoToken(ctx context.Context, dest, nome string, a sharedsvc.AlertaReuso, paraAdmin bool) error {
	dl, ok := ctx.Deadline()
	s.mu.Lock()
	s.chamdas = append(s.chamdas, sec09Chamada{dest: dest, nome: nome, alerta: a, paraAdmin: paraAdmin,
		deadline: dl, temPrazo: ok, ctxErr: ctx.Err(), chamadaEm: time.Now()})
	err := s.errPor[dest]
	s.mu.Unlock()
	if s.iniciou != nil {
		s.iniciou <- dest
		<-s.liberar
	}
	return err
}

func (s *sec09Sender) chamadas() []sec09Chamada {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sec09Chamada(nil), s.chamdas...)
}

func (s *sec09Sender) destinos() []string {
	var d []string
	for _, c := range s.chamadas() {
		d = append(d, c.dest)
	}
	return d
}

func (s *sec09Sender) reset() {
	s.mu.Lock()
	s.chamdas = nil
	s.mu.Unlock()
}

type sec09Lookup struct {
	mu    sync.Mutex
	users map[int64]*models.Usuario
	err   error
	nilOK bool // devolve (nil, nil)
	ctxs  []context.Context
}

func (l *sec09Lookup) GetByID(ctx context.Context, _ *sql.DB, id int64) (*models.Usuario, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ctxs = append(l.ctxs, ctx)
	if l.err != nil {
		return nil, l.err
	}
	if l.nilOK {
		return nil, nil
	}
	if u, ok := l.users[id]; ok {
		return u, nil
	}
	// Por padrão cada id tem um e-mail próprio.
	return &models.Usuario{ID: id, Nome: fmt.Sprintf("U%d", id), Email: fmt.Sprintf("u%d@exemplo.com", id)}, nil
}

type sec09Relogio struct {
	mu sync.Mutex
	t  time.Time
}

func (r *sec09Relogio) now() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.t
}

func (r *sec09Relogio) avancar(d time.Duration) {
	r.mu.Lock()
	r.t = r.t.Add(d)
	r.mu.Unlock()
}

// sec09Log captura o log padrão de forma segura para goroutines.
type sec09Log struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *sec09Log) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *sec09Log) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func sec09CapturarLog(t *testing.T) *sec09Log {
	t.Helper()
	l := &sec09Log{}
	anterior := log.Writer()
	log.SetOutput(l)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return l
}

var sec09T0 = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

// sec09Notifier monta o notifier com relógio manual, run síncrono e lookup
// fake em que o usuário 5 é a Maria.
func sec09Notifier(t *testing.T, admins []string) (*services.AlertaSegurancaNotifier, *sec09Sender, *sec09Lookup, *sec09Relogio) {
	t.Helper()
	s := &sec09Sender{}
	l := &sec09Lookup{users: map[int64]*models.Usuario{5: {ID: 5, Nome: "Maria", Email: sec09Email}}}
	r := &sec09Relogio{t: sec09T0}
	n := services.NewAlertaSegurancaNotifier(nil, s, admins)
	n.SetNow(r.now)
	n.SetRun(func(f func()) { f() })
	n.SetUsuarioLookup(l)
	return n, s, l, r
}

// ---------------------------------------------------------------------------
// envio básico
// ---------------------------------------------------------------------------

func TestSEC09_Notifier_PrimeiroAlerta_UsuarioEAdmins(t *testing.T) {
	logs := sec09CapturarLog(t)
	n, s, _, _ := sec09Notifier(t, sec09Admins)

	n.Notificar(5, sec09TokenID, sec09IP, sec09UA)

	cs := s.chamadas()
	require.Len(t, cs, 3)
	wantDest := []string{sec09Email, sec09Admin1, sec09Admin2}
	wantAdmin := []bool{false, true, true}
	for i, c := range cs {
		assert.Equal(t, wantDest[i], c.dest)
		assert.Equal(t, wantAdmin[i], c.paraAdmin)
		assert.Equal(t, "Maria", c.nome)
		assert.Equal(t, sharedsvc.AlertaReuso{UsuarioID: 5, EmailUsuario: sec09Email, IP: sec09IP,
			UserAgent: sec09UA, TokenID: sec09TokenID, Quando: sec09T0}, c.alerta)
	}
	out := logs.String()
	assert.Contains(t, out, "[auth][seguranca] alerta enviado: user_id=5 destinos=usuario,admin(2)")
	assert.NotContains(t, out, sec09Email)
}

func TestSEC09_Notifier_AdminsSaoCopiados(t *testing.T) {
	sec09CapturarLog(t)
	admins := []string{sec09Admin1}
	n, s, _, _ := sec09Notifier(t, admins)
	admins[0] = "trocado@evil.com"

	n.Notificar(5, 1, sec09IP, sec09UA)
	assert.Equal(t, []string{sec09Email, sec09Admin1}, s.destinos())
}

// ---------------------------------------------------------------------------
// dedup por usuário (30 min)
// ---------------------------------------------------------------------------

type sec09Passo struct {
	avancar time.Duration
	userID  int64
	envia   bool // false → suprimido (dedup)
}

func TestSEC09_Notifier_Dedup(t *testing.T) {
	casos := []struct {
		nome   string
		passos []sec09Passo
	}{
		{"mesmo usuário 10 min depois suprimido, aos 31 min envia", []sec09Passo{
			{0, 5, true}, {10 * time.Minute, 5, false}, {21 * time.Minute, 5, true}}},
		{"limite exato de 30 min envia; 29m59s não", []sec09Passo{
			{0, 5, true}, {30*time.Minute - time.Second, 5, false}, {time.Second, 5, true}}},
		{"usuários diferentes não se deduplicam", []sec09Passo{
			{0, 5, true}, {time.Minute, 6, true}, {time.Minute, 7, true}, {time.Minute, 5, false}, {0, 6, false}}},
		{"supressão não renova a marcação (conta desde o envio)", []sec09Passo{
			{0, 5, true}, {20 * time.Minute, 5, false}, {10 * time.Minute, 5, true}}},
		// Poda de vencidos: o alerta do 6 aos 31 min poda a marcação do 5;
		// o 5 volta a receber normalmente.
		{"poda de vencidos", []sec09Passo{
			{0, 5, true}, {31 * time.Minute, 6, true}, {time.Minute, 5, true}, {time.Minute, 6, false}}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			logs := sec09CapturarLog(t)
			n, s, _, r := sec09Notifier(t, nil)
			for i, p := range c.passos {
				r.avancar(p.avancar)
				antes := len(s.chamadas())
				antesLog := len(logs.String())
				n.Notificar(p.userID, int64(100+i), sec09IP, sec09UA)
				novo := logs.String()[antesLog:]
				if p.envia {
					assert.Equal(t, antes+1, len(s.chamadas()), "passo %d deveria enviar", i)
					assert.Contains(t, novo, fmt.Sprintf("alerta enviado: user_id=%d destinos=usuario", p.userID))
				} else {
					assert.Equal(t, antes, len(s.chamadas()), "passo %d deveria ser suprimido", i)
					assert.Contains(t, novo, fmt.Sprintf("[auth][seguranca] alerta suprimido (dedup): user_id=%d", p.userID))
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// teto global (20 por janela fixa de 1 h)
// ---------------------------------------------------------------------------

func TestSEC09_Notifier_Teto(t *testing.T) {
	require.Equal(t, 20, services.AlertaTetoPorHora)
	require.Equal(t, 30*time.Minute, services.AlertaDedupJanela)
	require.Equal(t, 4, services.AlertaMaxEnviosSimultaneos)

	casos := []struct {
		nome string
		run  func(t *testing.T, n *services.AlertaSegurancaNotifier, s *sec09Sender, r *sec09Relogio, logs *sec09Log)
	}{
		{"21º em 1h suprimido pelo teto", func(t *testing.T, n *services.AlertaSegurancaNotifier, s *sec09Sender, r *sec09Relogio, logs *sec09Log) {
			for id := int64(1); id <= 20; id++ {
				n.Notificar(id, id, sec09IP, sec09UA)
				r.avancar(time.Minute)
			}
			require.Len(t, s.chamadas(), 20)
			n.Notificar(21, 21, sec09IP, sec09UA)
			assert.Len(t, s.chamadas(), 20)
			assert.Contains(t, logs.String(), "[auth][seguranca] alerta suprimido (teto): user_id=21")
		}},
		{"janela reinicia após 1h do início (janela fixa)", func(t *testing.T, n *services.AlertaSegurancaNotifier, s *sec09Sender, r *sec09Relogio, logs *sec09Log) {
			for id := int64(1); id <= 20; id++ {
				n.Notificar(id, id, sec09IP, sec09UA)
			}
			r.avancar(59*time.Minute + 59*time.Second)
			n.Notificar(21, 21, sec09IP, sec09UA)
			assert.Len(t, s.chamadas(), 20, "ainda dentro da janela")
			r.avancar(time.Second) // exatamente 1h do início
			n.Notificar(21, 21, sec09IP, sec09UA)
			assert.Len(t, s.chamadas(), 21, "janela nova")
			// e a nova janela conta de novo até 20
			for id := int64(22); id <= 40; id++ {
				n.Notificar(id, id, sec09IP, sec09UA)
			}
			assert.Len(t, s.chamadas(), 40)
			n.Notificar(41, 41, sec09IP, sec09UA)
			assert.Len(t, s.chamadas(), 40)
			assert.Contains(t, logs.String(), "alerta suprimido (teto): user_id=41")
		}},
		{"suprimido pelo teto não marca dedup", func(t *testing.T, n *services.AlertaSegurancaNotifier, s *sec09Sender, r *sec09Relogio, logs *sec09Log) {
			for id := int64(1); id <= 20; id++ {
				n.Notificar(id, id, sec09IP, sec09UA)
			}
			n.Notificar(99, 1, sec09IP, sec09UA) // teto
			r.avancar(time.Hour)
			antesLog := len(logs.String())
			n.Notificar(99, 2, sec09IP, sec09UA) // nova janela, sem dedup
			assert.Len(t, s.chamadas(), 21)
			assert.Contains(t, logs.String()[antesLog:], "alerta enviado: user_id=99")
		}},
		{"dedup não consome o teto", func(t *testing.T, n *services.AlertaSegurancaNotifier, s *sec09Sender, r *sec09Relogio, logs *sec09Log) {
			n.Notificar(1, 1, sec09IP, sec09UA)
			for i := 0; i < 30; i++ {
				n.Notificar(1, 1, sec09IP, sec09UA) // dedup
			}
			for id := int64(2); id <= 20; id++ {
				n.Notificar(id, id, sec09IP, sec09UA)
			}
			assert.Len(t, s.chamadas(), 20)
			assert.NotContains(t, logs.String(), "(teto)")
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			logs := sec09CapturarLog(t)
			n, s, _, r := sec09Notifier(t, nil)
			c.run(t, n, s, r, logs)
		})
	}
}

// ---------------------------------------------------------------------------
// destinatários conforme lookup / admins
// ---------------------------------------------------------------------------

func TestSEC09_Notifier_Destinatarios(t *testing.T) {
	casos := []struct {
		nome       string
		admins     []string
		lookup     func(l *sec09Lookup)
		wantDest   []string
		wantNome   string
		wantLog    []string
		wantSemLog []string
	}{
		{
			nome:     "GetByID ErrNotFound → só admins",
			admins:   sec09Admins,
			lookup:   func(l *sec09Lookup) { l.err = repositories.ErrNotFound },
			wantDest: []string{sec09Admin1, sec09Admin2},
			wantLog: []string{"[auth][seguranca] alerta: usuário não carregado (só admins): user_id=5",
				"alerta enviado: user_id=5 destinos=admin(2)"},
		},
		{
			nome:     "GetByID erro genérico → só admins",
			admins:   []string{sec09Admin1},
			lookup:   func(l *sec09Lookup) { l.err = errors.New("db fora") },
			wantDest: []string{sec09Admin1},
			wantLog:  []string{"usuário não carregado (só admins): user_id=5: db fora", "destinos=admin(1)"},
		},
		{
			nome:       "e-mail vazio → só admins",
			admins:     sec09Admins,
			lookup:     func(l *sec09Lookup) { l.users[5] = &models.Usuario{ID: 5, Nome: "Maria", Email: ""} },
			wantDest:   []string{sec09Admin1, sec09Admin2},
			wantNome:   "Maria",
			wantLog:    []string{"alerta enviado: user_id=5 destinos=admin(2)"},
			wantSemLog: []string{"não carregado", "falha"},
		},
		{
			nome:     "e-mail só com espaços → só admins",
			admins:   sec09Admins,
			lookup:   func(l *sec09Lookup) { l.users[5] = &models.Usuario{ID: 5, Nome: "Maria", Email: "   "} },
			wantDest: []string{sec09Admin1, sec09Admin2},
			wantNome: "Maria",
			wantLog:  []string{"destinos=admin(2)"},
		},
		{
			nome:     "lookup devolve (nil, nil) → só admins",
			admins:   sec09Admins,
			lookup:   func(l *sec09Lookup) { l.nilOK = true },
			wantDest: []string{sec09Admin1, sec09Admin2},
			wantLog:  []string{"destinos=admin(2)"},
		},
		{
			nome:   "e-mail com espaços nas pontas é aparado",
			admins: nil,
			lookup: func(l *sec09Lookup) {
				l.users[5] = &models.Usuario{ID: 5, Nome: "Maria", Email: "  " + sec09Email + " "}
			},
			wantDest: []string{sec09Email},
			wantNome: "Maria",
			wantLog:  []string{"alerta enviado: user_id=5 destinos=usuario\n"},
		},
		{
			nome:       "sem admins → só usuário",
			admins:     nil,
			lookup:     func(*sec09Lookup) {},
			wantDest:   []string{sec09Email},
			wantNome:   "Maria",
			wantLog:    []string{"alerta enviado: user_id=5 destinos=usuario\n"},
			wantSemLog: []string{"admin("},
		},
		{
			nome:       "sem admins e usuário não carregado → sem destinatários",
			admins:     nil,
			lookup:     func(l *sec09Lookup) { l.err = repositories.ErrNotFound },
			wantDest:   nil,
			wantLog:    []string{"[auth][seguranca] alerta sem destinatários entregues: user_id=5"},
			wantSemLog: []string{"alerta enviado"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			logs := sec09CapturarLog(t)
			n, s, l, _ := sec09Notifier(t, c.admins)
			c.lookup(l)

			n.Notificar(5, sec09TokenID, sec09IP, sec09UA)

			assert.Equal(t, c.wantDest, s.destinos())
			for _, ch := range s.chamadas() {
				assert.Equal(t, c.wantNome, ch.nome)
				assert.Equal(t, ch.dest != sec09Email, ch.paraAdmin)
			}
			out := logs.String()
			for _, w := range c.wantLog {
				assert.Contains(t, out, w)
			}
			for _, w := range c.wantSemLog {
				assert.NotContains(t, out, w)
			}
			assert.NotContains(t, out, sec09Email)
		})
	}
}

// ---------------------------------------------------------------------------
// falhas do sender
// ---------------------------------------------------------------------------

func TestSEC09_Notifier_FalhaDoSender(t *testing.T) {
	errEco := fmt.Errorf("services: smtp rcpt to: 550 5.1.1 <%s>: mailbox %s unavailable", sec09Email, sec09Email)
	casos := []struct {
		nome       string
		errPor     map[string]error
		wantLog    []string
		wantSemLog []string
	}{
		{
			nome:   "falha só no usuário (erro ecoa o e-mail) → oculta e segue para admins",
			errPor: map[string]error{sec09Email: errEco},
			wantLog: []string{
				"[auth][seguranca] falha ao enviar alerta: user_id=5: services: smtp rcpt to: 550 5.1.1 <[email-usuario]>: mailbox [email-usuario] unavailable",
				"alerta enviado: user_id=5 destinos=admin(2)",
			},
		},
		{
			nome:    "falha em um admin → admin(1)",
			errPor:  map[string]error{sec09Admin1: errors.New("smtp dial: timeout")},
			wantLog: []string{"falha ao enviar alerta: user_id=5: smtp dial: timeout", "destinos=usuario,admin(1)"},
		},
		{
			nome:    "erro do admin que ecoa o e-mail do usuário também é ocultado",
			errPor:  map[string]error{sec09Admin2: fmt.Errorf("rejeitado: corpo cita %s", sec09Email)},
			wantLog: []string{"falha ao enviar alerta: user_id=5: rejeitado: corpo cita [email-usuario]", "destinos=usuario,admin(1)"},
		},
		{
			nome: "tudo falha → sem destinatários entregues",
			errPor: map[string]error{sec09Email: errEco, sec09Admin1: errors.New("x"),
				sec09Admin2: errors.New("y")},
			wantLog:    []string{"alerta sem destinatários entregues: user_id=5"},
			wantSemLog: []string{"alerta enviado"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			logs := sec09CapturarLog(t)
			n, s, _, r := sec09Notifier(t, sec09Admins)
			s.errPor = c.errPor

			n.Notificar(5, sec09TokenID, sec09IP, sec09UA)
			assert.Len(t, s.chamadas(), 3, "tenta todos os destinos mesmo com falha")

			out := logs.String()
			for _, w := range c.wantLog {
				assert.Contains(t, out, w)
			}
			for _, w := range c.wantSemLog {
				assert.NotContains(t, out, w)
			}
			assert.NotContains(t, out, sec09Email, "log nunca contém o e-mail do usuário")

			// Marcação mantida: falha não desmarca o dedup.
			r.avancar(time.Minute)
			s.reset()
			n.Notificar(5, sec09TokenID+1, sec09IP, sec09UA)
			assert.Empty(t, s.chamadas())
			assert.Contains(t, logs.String(), "alerta suprimido (dedup): user_id=5")
		})
	}
}

// Usuário não carregado (sem e-mail conhecido) e falha no admin: o erro é
// logado como veio (nada a ocultar) e o alerta fica sem destinatários.
func TestSEC09_Notifier_FalhaAdmin_UsuarioNaoCarregado(t *testing.T) {
	logs := sec09CapturarLog(t)
	n, s, l, _ := sec09Notifier(t, []string{sec09Admin1})
	l.err = repositories.ErrNotFound
	s.errPor = map[string]error{sec09Admin1: errors.New("smtp rcpt to: 550 caixa cheia")}

	n.Notificar(5, sec09TokenID, sec09IP, sec09UA)

	assert.Equal(t, []string{sec09Admin1}, s.destinos())
	out := logs.String()
	assert.Contains(t, out, "[auth][seguranca] falha ao enviar alerta: user_id=5: smtp rcpt to: 550 caixa cheia")
	assert.Contains(t, out, "alerta sem destinatários entregues: user_id=5")
	assert.NotContains(t, out, "[email-usuario]")
}

// ---------------------------------------------------------------------------
// contexto próprio (~30 s, independente, não cancelado)
// ---------------------------------------------------------------------------

func TestSEC09_Notifier_ContextoProprio(t *testing.T) {
	sec09CapturarLog(t)
	n, s, l, _ := sec09Notifier(t, sec09Admins)

	antes := time.Now()
	n.Notificar(5, sec09TokenID, sec09IP, sec09UA)
	depois := time.Now()

	cs := s.chamadas()
	require.Len(t, cs, 3)
	for _, c := range cs {
		require.True(t, c.temPrazo, "ctx do envio deve ter deadline")
		assert.NoError(t, c.ctxErr, "ctx não pode estar cancelado no envio")
		assert.False(t, c.deadline.Before(antes.Add(30*time.Second)), "deadline %v < início+30s", c.deadline)
		assert.False(t, c.deadline.After(depois.Add(30*time.Second)), "deadline %v > fim+30s", c.deadline)
	}
	assert.Equal(t, cs[0].deadline, cs[2].deadline, "um único ctx para todos os envios do alerta")

	require.Len(t, l.ctxs, 1)
	dl, ok := l.ctxs[0].Deadline()
	require.True(t, ok)
	assert.Equal(t, cs[0].deadline, dl, "lookup usa o mesmo ctx")
	// Após o envio o ctx próprio é liberado (cancel no defer).
	assert.Error(t, l.ctxs[0].Err())
}

// O relógio injetado (now) só afeta dedup/teto e o horário do alerta; o
// prazo do envio usa o tempo real.
func TestSEC09_Notifier_RelogioInjetadoNaoAfetaPrazo(t *testing.T) {
	sec09CapturarLog(t)
	n, s, _, r := sec09Notifier(t, nil)
	r.avancar(-24 * 365 * time.Hour)
	n.Notificar(5, 1, sec09IP, sec09UA)
	cs := s.chamadas()
	require.Len(t, cs, 1)
	assert.NoError(t, cs[0].ctxErr)
	assert.Equal(t, r.now(), cs[0].alerta.Quando)
	assert.WithinDuration(t, time.Now().Add(30*time.Second), cs[0].deadline, 5*time.Second)
}

// ---------------------------------------------------------------------------
// concorrência: no máximo 4 envios simultâneos; o 5º é descartado
// ---------------------------------------------------------------------------

func TestSEC09_Notifier_FilaCheia(t *testing.T) {
	logs := sec09CapturarLog(t)
	s := &sec09Sender{iniciou: make(chan string, 16), liberar: make(chan struct{})}
	r := &sec09Relogio{t: sec09T0}
	fim := make(chan struct{}, 16)
	n := services.NewAlertaSegurancaNotifier(nil, s, nil)
	n.SetNow(r.now)
	n.SetUsuarioLookup(&sec09Lookup{users: map[int64]*models.Usuario{}})
	// Executor assíncrono que avisa quando f terminou (inclusive a liberação
	// do semáforo, feita por defer dentro de f).
	n.SetRun(func(f func()) {
		go func() {
			f()
			fim <- struct{}{}
		}()
	})

	for id := int64(1); id <= 4; id++ {
		n.Notificar(id, id, sec09IP, sec09UA)
	}
	for i := 0; i < 4; i++ {
		<-s.iniciou // os 4 envios estão em andamento (bloqueados)
	}

	n.Notificar(5, 5, sec09IP, sec09UA)
	assert.Contains(t, logs.String(), "[auth][seguranca] alerta suprimido (fila cheia): user_id=5")
	assert.Len(t, s.chamadas(), 4, "o 5º não chega ao sender")

	// Libera os 4 e espera terminarem de verdade.
	for i := 0; i < 4; i++ {
		s.liberar <- struct{}{}
	}
	for i := 0; i < 4; i++ {
		<-fim
	}
	out := logs.String()
	for id := 1; id <= 4; id++ {
		assert.Contains(t, out, fmt.Sprintf("alerta enviado: user_id=%d destinos=usuario", id))
	}

	// Com a fila livre, um novo usuário volta a ser enviado.
	n.Notificar(6, 6, sec09IP, sec09UA)
	assert.Equal(t, "u6@exemplo.com", <-s.iniciou)
	s.liberar <- struct{}{}
	<-fim
	assert.Len(t, s.chamadas(), 5)
	assert.NotContains(t, logs.String(), "fila cheia): user_id=6")
}

// Descarte por fila cheia desfaz a reserva: liberado o semáforo, o MESMO
// usuário volta a ser enviado de imediato (sem dedup) e o descarte não
// consumiu vaga do teto.
func TestSEC09_Notifier_FilaCheia_DesfazReserva(t *testing.T) {
	logs := sec09CapturarLog(t)
	s := &sec09Sender{iniciou: make(chan string, 64), liberar: make(chan struct{})}
	r := &sec09Relogio{t: sec09T0}
	fim := make(chan struct{}, 64)
	n := services.NewAlertaSegurancaNotifier(nil, s, nil)
	n.SetNow(r.now)
	n.SetUsuarioLookup(&sec09Lookup{users: map[int64]*models.Usuario{}})
	n.SetRun(func(f func()) {
		go func() {
			f()
			fim <- struct{}{}
		}()
	})
	liberarTodos := func(k int) {
		for i := 0; i < k; i++ {
			s.liberar <- struct{}{}
		}
		for i := 0; i < k; i++ {
			<-fim
		}
	}

	// Ocupa as 4 vagas (usuários 1..4 → 4 do teto).
	for id := int64(1); id <= 4; id++ {
		n.Notificar(id, id, sec09IP, sec09UA)
	}
	for i := 0; i < 4; i++ {
		<-s.iniciou
	}

	// Usuário 5 descartado por fila cheia várias vezes: nenhuma consome teto.
	for i := 0; i < 10; i++ {
		n.Notificar(5, 5, sec09IP, sec09UA)
	}
	assert.Contains(t, logs.String(), "[auth][seguranca] alerta suprimido (fila cheia): user_id=5")
	assert.Len(t, s.chamadas(), 4)
	liberarTodos(4)

	// Mesmo usuário, mesmo instante: enviado de imediato (sem dedup).
	antesLog := len(logs.String())
	n.Notificar(5, 5, sec09IP, sec09UA)
	assert.Equal(t, "u5@exemplo.com", <-s.iniciou)
	liberarTodos(1)
	novo := logs.String()[antesLog:]
	assert.NotContains(t, novo, "suprimido")
	assert.Contains(t, novo, "alerta enviado: user_id=5 destinos=usuario")

	// Teto: 5 usados (1..5); os descartes não contaram, então cabem mais 15
	// (6..20) e só o 21º é suprimido pelo teto.
	for id := int64(6); id <= 20; id++ {
		n.Notificar(id, id, sec09IP, sec09UA)
		<-s.iniciou
		liberarTodos(1)
	}
	assert.Len(t, s.chamadas(), 20)
	assert.NotContains(t, logs.String(), "(teto)")
	n.Notificar(21, 21, sec09IP, sec09UA)
	assert.Len(t, s.chamadas(), 20)
	assert.Contains(t, logs.String(), "alerta suprimido (teto): user_id=21")

	// E o 5 segue deduplicado após o envio real.
	n.Notificar(5, 6, sec09IP, sec09UA)
	assert.Contains(t, logs.String(), "alerta suprimido (dedup): user_id=5")
}

// Construtor com dependências padrão: executor em goroutine, relógio real e
// UsuarioRepository real (sqlmock).
func TestSEC09_Notifier_DependenciasPadrao(t *testing.T) {
	logs := sec09CapturarLog(t)
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	casos := []struct {
		nome     string
		userID   int64
		setup    func()
		wantDest []string
	}{
		{"usuário encontrado → usuário + admin", 5, func() {
			mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.id\s+=\s+\?\s+LIMIT\s+1`).
				WithArgs(int64(5)).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
					AddRow(int64(5), "Maria", sec09Email, "hash", "normal", nil, true, false, time.Now(), time.Now(), nil, nil))
		}, []string{sec09Email, sec09Admin1}},
		{"usuário inexistente (sql.ErrNoRows) → só admin", 6, func() {
			mock.ExpectQuery(`WHERE\s+u\.id\s+=\s+\?`).WithArgs(int64(6)).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))
		}, []string{sec09Admin1}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			s := &sec09Sender{iniciou: make(chan string, 4), liberar: make(chan struct{}, 4)}
			for range c.wantDest {
				s.liberar <- struct{}{}
			}
			n := services.NewAlertaSegurancaNotifier(db, s, []string{sec09Admin1})
			c.setup()

			n.Notificar(c.userID, 1, sec09IP, sec09UA)
			for _, want := range c.wantDest {
				assert.Equal(t, want, <-s.iniciou)
			}
			assert.WithinDuration(t, time.Now(), s.chamadas()[0].alerta.Quando, 5*time.Second)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
	assert.NotContains(t, logs.String(), sec09Email)
}
