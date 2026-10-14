package vlog_test

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

// LOG-02: valida em runtime que os logs verbose seguem o formato
// "[arquivo.go] [Funcao] descrição" e nunca expõem senha, hash, token, IP ou
// User-Agent; e que com a flag desligada nada é emitido. ARQ-03: o hash, a
// política e o gerador de senha viraram código puro (identidadecrypto e
// domínio, sem vlog); os casos passaram aos adapters de identidademysql, que
// leem e gravam esses segredos (vlog nível 4: só operação e contagens).

var linhaVlog = regexp.MustCompile(`^\[(usuarios|historico|refresh_tokens)\.go\] \[[A-Za-z][A-Za-z0-9_.]*\] \S.*$`)

func capturar(t *testing.T, ligado bool, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags, prevEnabled := log.Writer(), log.Flags(), vlog.Enabled()
	log.SetOutput(&buf)
	log.SetFlags(0)
	vlog.SetEnabled(ligado)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		vlog.SetEnabled(prevEnabled)
	})
	fn()
	return buf.String()
}

type caso struct {
	nome    string
	arquivo string
	// exec roda a função instrumentada e devolve os valores sensíveis que
	// NÃO podem aparecer no log.
	exec func(t *testing.T) []string
}

func casos() []caso {
	const hash = "$argon2id$v=19$m=64,t=1,p=1$c2FsdHNhbHRzYWx0$ZGlnZXN0ZGlnZXN0ZGlnZXN0"
	const email = "fulano.segredo@empresa.com"
	linhaUsuario := func(m sqlmock.Sqlmock) {
		m.ExpectQuery("FROM usuarios u").WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo",
			"deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(1), "Fulano", email, hash, "normal", nil, true, false, time.Now(), time.Now(), nil, nil))
	}
	return []caso{
		{"Usuarios.PorEmail", "usuarios.go", func(t *testing.T) []string {
			db, m := novoMock(t)
			linhaUsuario(m)
			if _, err := identidademysql.NovoUsuarios(db).PorEmail(context.Background(), email); err != nil {
				t.Fatalf("PorEmail: %v", err)
			}
			return []string{email, hash}
		}},
		{"Usuarios.AtualizarSenha", "usuarios.go", func(t *testing.T) []string {
			db, m := novoMock(t)
			m.ExpectExec("UPDATE usuarios SET password_hash").WillReturnResult(sqlmock.NewResult(0, 1))
			if err := identidademysql.NovoUsuarios(db).AtualizarSenha(context.Background(), 1, identidade.NovoHashSenha(hash), false); err != nil {
				t.Fatalf("AtualizarSenha: %v", err)
			}
			return []string{hash}
		}},
		{"HistoricoSenhas.DoUsuario", "historico.go", func(t *testing.T) []string {
			db, m := novoMock(t)
			m.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
			m.ExpectQuery("FROM senha_historico sh").WillReturnRows(sqlmock.NewRows([]string{"id", "usuario_id", "resetado_por_id", "senha_hash_anterior",
				"ip_origem", "user_agent", "tipo_reset", "created_at", "usuario_nome", "resetado_por_nome"}).
				AddRow(int64(1), int64(1), nil, hash, "10.9.8.7", "UA-Segredo/1", "usuario", time.Now(), "Fulano", nil))
			if _, err := identidademysql.NovoHistoricoSenhas(db).DoUsuario(context.Background(), 1, identidade.OrdenacaoHistoricoPadrao(), kernel.NovaPagina(1, 2)); err != nil {
				t.Fatalf("DoUsuario: %v", err)
			}
			return []string{hash, "10.9.8.7", "UA-Segredo/1"}
		}},
		{"RefreshTokens.PorHash", "refresh_tokens.go", func(t *testing.T) []string {
			db, m := novoMock(t)
			tokenHash := "abc123hashdotokenmuitosecreto456"
			m.ExpectQuery("FROM refresh_tokens").WillReturnRows(sqlmock.NewRows([]string{"id", "usuario_id", "token_hash", "expires_at", "revoked_at",
				"ip_origem", "user_agent", "revoked_reason"}).AddRow(int64(9), int64(1), tokenHash, time.Now(), nil, "10.9.8.7", "UA-Segredo/1", nil))
			if _, err := identidademysql.NovoRefreshTokens(db).PorHash(context.Background(), tokenHash); err != nil {
				t.Fatalf("PorHash: %v", err)
			}
			return []string{tokenHash, "10.9.8.7", "UA-Segredo/1"}
		}},
	}
}

func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, m, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, m
}

func TestVlogRuntimeLigadoFormatoESemVazamento(t *testing.T) {
	for _, c := range casos() {
		t.Run(c.nome, func(t *testing.T) {
			var sensiveis []string
			out := capturar(t, true, func() { sensiveis = c.exec(t) })
			if strings.TrimSpace(out) == "" {
				t.Fatal("esperava logs verbose com a flag ligada")
			}
			viuArquivo := false
			for _, linha := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
				if !linhaVlog.MatchString(linha) {
					t.Errorf("linha fora do formato [arquivo] [funcao] msg: %q", linha)
				}
				if strings.HasPrefix(linha, "["+c.arquivo+"]") {
					viuArquivo = true
				}
			}
			if !viuArquivo {
				t.Errorf("nenhuma linha com [%s]", c.arquivo)
			}
			for _, s := range sensiveis {
				if s != "" && strings.Contains(out, s) {
					t.Errorf("valor sensível vazou no log verbose (len=%d)", len(s))
				}
			}
		})
	}
}

func TestVlogRuntimeDesligadoNaoEmite(t *testing.T) {
	for _, c := range casos() {
		t.Run(c.nome, func(t *testing.T) {
			out := capturar(t, false, func() { _ = c.exec(t) })
			if out != "" {
				t.Fatalf("com a flag desligada nada deveria ser emitido; veio %d bytes", len(out))
			}
		})
	}
}

func TestVlogMaskEmailNaoExpoeUsuario(t *testing.T) {
	casosEmail := []struct{ in, want string }{
		{"ana.silva@empresa.com", "a***@empresa.com"},
		{"x@y.com", "x***@y.com"},
		{"@semusuario.com", "***"},
		{"sem-arroba", "***"},
		{"", "***"},
	}
	for _, c := range casosEmail {
		t.Run(c.in, func(t *testing.T) {
			if got := vlog.MaskEmail(c.in); got != c.want {
				t.Fatalf("MaskEmail(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
