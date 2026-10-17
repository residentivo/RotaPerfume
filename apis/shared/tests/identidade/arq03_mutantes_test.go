package identidade_test

// ARQ-03 (validação do TestBrain): testes que matam mutantes de segurança
// que sobreviviam à suíte do BackBrain:
//   - captcha depois do rate limit no login e no reset (I-13);
//   - login de e-mail inexistente sem o hash descartável (I-16), tanto no
//     caso de uso quanto no Hasher (descartável virando no-op);
//   - log "decisão criado" do CriarUsuario emitido sem verbose (o ramo
//     não-verbose do TestBUG11 ainda assere o formato antigo).

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

// limitadorEspiao bloqueia todas as chaves e registra cada chamada.
type limitadorEspiao struct {
	mu        sync.Mutex
	chamadas  []string
	bloqueado bool
}

func (l *limitadorEspiao) anotar(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.chamadas = append(l.chamadas, s)
}

func (l *limitadorEspiao) Bloqueado(chave string) (bool, time.Duration) {
	l.anotar("bloqueado:" + chave)
	return l.bloqueado, 30 * time.Second
}
func (l *limitadorEspiao) RegistrarFalha(chave string)   { l.anotar("falha:" + chave) }
func (l *limitadorEspiao) RegistrarSucesso(chave string) { l.anotar("sucesso:" + chave) }

func casosComLimitador(a *ambiente, l identidadeapp.Limitador) *identidadeapp.Casos {
	d := a.deps()
	d.Limites = identidadeapp.Limites{Login: l, Refresh: l, Reset: l}
	return identidadeapp.Novo(d, identidadeapp.Opcoes{})
}

// TestARQ03_CaptchaAntesDoRateLimit (I-13): com o IP/conta já bloqueado, um
// captcha inválido responde captcha inválido (400), não 429, e o rate limit
// nem é consultado. Com captcha válido o bloqueio vale (sanidade).
func TestARQ03_CaptchaAntesDoRateLimit(t *testing.T) {
	u := usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal)
	operacoes := []struct {
		nome   string
		chamar func(c *identidadeapp.Casos, captcha string) error
	}{
		{"login", func(c *identidadeapp.Casos, captcha string) error {
			_, err := c.Login(ctx, identidadeapp.ComandoLogin{Email: "a@b.c", Senha: identidade.NovaSenha("Atual#123"),
				Captcha: captcha, Origem: identidadeapp.Origem{IP: "1.2.3.4"}})
			return err
		}},
		{"reset-password", func(c *identidadeapp.Casos, captcha string) error {
			return c.TrocarSenha(ctx, principal(2, kernel.RoleNormal), identidadeapp.ComandoTrocarSenha{
				SenhaAtual: identidade.NovaSenha("Atual#123"), NovaSenha: identidade.NovaSenha("Nova#12345"),
				Captcha: captcha, Origem: identidadeapp.Origem{IP: "1.2.3.4"}})
		}},
	}
	captchas := []struct {
		nome         string
		token        string
		wantErr      error
		wantConsulta bool
	}{
		{"captcha ausente com IP bloqueado → captcha inválido", "", identidade.ErrCaptchaInvalido, false},
		{"captcha só espaços com IP bloqueado → captcha inválido", "   ", identidade.ErrCaptchaInvalido, false},
		{"captcha válido com IP bloqueado → 429", "ok", identidade.ErrMuitasTentativas, true},
	}
	for _, op := range operacoes {
		for _, cp := range captchas {
			t.Run(op.nome+"/"+cp.nome, func(t *testing.T) {
				l := &limitadorEspiao{bloqueado: true}
				err := op.chamar(casosComLimitador(novoAmbiente(u), l), cp.token)
				require.Error(t, err)
				assert.ErrorIs(t, err, cp.wantErr)
				var b *kernel.Bloqueio
				if cp.wantConsulta {
					assert.ErrorAs(t, err, &b)
					assert.NotEmpty(t, l.chamadas)
				} else {
					assert.False(t, errors.As(err, &b), "captcha inválido nunca vira 429")
					assert.NotErrorIs(t, err, identidade.ErrMuitasTentativas)
					assert.Empty(t, l.chamadas, "o rate limit não é consultado nem consumido antes do captcha")
				}
			})
		}
	}
}

// hasherEspiao conta as chamadas de verificação e do hash descartável.
type hasherEspiao struct {
	*hasherFake
	descartaveis, verificacoes int
}

func (h *hasherEspiao) ExecutarDescartavel(identidade.Senha) { h.descartaveis++ }
func (h *hasherEspiao) Verificar(hash identidade.HashSenha, s identidade.Senha) bool {
	h.verificacoes++
	return h.hasherFake.Verificar(hash, s)
}

// TestARQ03_Login_AntiEnumeracao_HashDescartavel (I-16): e-mail inexistente
// roda o hash descartável (mesmo custo de um login real) e devolve o mesmo
// erro da senha errada.
func TestARQ03_Login_AntiEnumeracao_HashDescartavel(t *testing.T) {
	casos := []struct {
		nome             string
		email, senha     string
		wantDescartavel  int
		wantVerificacoes int
		wantErr          error
	}{
		{"e-mail inexistente", "nao@existe.c", "Atual#123", 1, 0, identidade.ErrCredenciaisInvalidas},
		{"senha errada", "a@b.c", "Errada#123", 0, 1, identidade.ErrCredenciaisInvalidas},
		{"senha certa", "a@b.c", "Atual#123", 0, 1, nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
			h := &hasherEspiao{hasherFake: a.hasher}
			d := a.deps()
			d.Hasher = h
			_, err := identidadeapp.Novo(d, identidadeapp.Opcoes{}).Login(ctx, identidadeapp.ComandoLogin{
				Email: c.email, Senha: identidade.NovaSenha(c.senha), Captcha: "ok", Origem: identidadeapp.Origem{IP: "1.2.3.4"}})
			if c.wantErr == nil {
				require.NoError(t, err)
			} else {
				assert.Equal(t, c.wantErr, err, "inexistente e senha errada: mesmo erro")
			}
			assert.Equal(t, c.wantDescartavel, h.descartaveis, "hash descartável")
			assert.Equal(t, c.wantVerificacoes, h.verificacoes, "verificação real")
		})
	}
}

// TestARQ03_Hasher_DescartavelCustaComoVerificar (I-16): o descartável do
// Hasher real roda um Argon2id de verdade. Parâmetros pesados o bastante
// para um no-op ficar ordens de grandeza abaixo; a margem (1/10) absorve
// ruído de agendamento.
func TestARQ03_Hasher_DescartavelCustaComoVerificar(t *testing.T) {
	hs := config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 16 * 1024, Iteracoes: 2, Paralelismo: 1}
	h := identidadecrypto.NovoHasher(hs)
	senha := identidade.NovaSenha("Atual#123")
	hash, err := h.Gerar(senha)
	require.NoError(t, err)

	// Medição intercalada e pelo MÍNIMO de cada lado: o mínimo é o custo
	// intrínseco (picos de carga só aumentam o tempo), e intercalar expõe os
	// dois à mesma carga. Um no-op mede ~0.
	medir := func(fn func()) time.Duration {
		ini := time.Now()
		fn()
		return time.Since(ini)
	}
	verificar, descartavel := time.Duration(1<<62), time.Duration(1<<62)
	for i := 0; i < 7; i++ {
		verificar = min(verificar, medir(func() { require.True(t, h.Verificar(hash, senha)) }))
		descartavel = min(descartavel, medir(func() { h.ExecutarDescartavel(senha) }))
	}
	assert.GreaterOrEqual(t, descartavel, verificar/10,
		"descartável=%s verificar=%s: o descartável precisa custar um Argon2id", descartavel, verificar)
}

// TestARQ03_CriarUsuario_LogDeCriacaoSoComVerbose: a linha "decisão criado"
// é vlog (nível 3): sem verbose não sai; com verbose sai com o e-mail
// mascarado (§6.3) e nunca com a senha gerada.
func TestARQ03_CriarUsuario_LogDeCriacaoSoComVerbose(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		t.Run(map[bool]string{false: "sem verbose", true: "com verbose"}[verbose], func(t *testing.T) {
			var buf bytes.Buffer
			prevOut, prevFlags, prevVlog := log.Writer(), log.Flags(), vlog.Enabled()
			log.SetOutput(&buf)
			log.SetFlags(0)
			vlog.SetEnabled(verbose)
			t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags); vlog.SetEnabled(prevVlog) })

			a := novoAmbiente()
			_, enviado, err := a.casos().CriarUsuario(ctx, principal(1, kernel.RoleAdmin),
				identidade.DadosUsuario{Nome: "Fulano", Email: "Fulano@Test.com", Role: "normal"})
			require.NoError(t, err)
			require.True(t, enviado)
			saida := buf.String()

			assert.Equal(t, verbose, strings.Contains(saida, "decisão criado"), "log de criação só com verbose:\n%s", saida)
			if verbose {
				assert.Contains(t, saida, "[usuarios.go] [Identidade.CriarUsuario] decisão criado")
				assert.Contains(t, saida, "email=f***@test.com")
			}
			assert.NotContains(t, saida, "fulano@test.com", "e-mail nunca em claro no log")
			assert.NotContains(t, saida, "Gerada#1234567", "senha gerada nunca no log")
		})
	}
}
