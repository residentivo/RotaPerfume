package config_test

// SEC-11 (Lote 12): DB_USUARIO/DB_SENHA são obrigatórios e sem default.
//   - Load() devolve config.ErrCredenciaisDB (erro, não panic; cfg nil) se
//     algum dos dois estiver ausente/vazio ou o usuário for só espaços.
//   - A mensagem nunca carrega os valores.
//   - A checagem roda no fim: os erros de JWT_SECRET/JWT_TTL/PASSWORD_PEPPER e
//     das durações de refresh têm prioridade.
//   - LoadSemCredenciaisDB() (seedusers -dry-run/-no-exec) não exige DB, mas
//     mantém as demais validações.

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

const (
	sec11Usuario = "usuario-sec11"
	sec11Senha   = "senha-sec11-nao-pode-vazar"
)

// sec11Env define as variáveis do caso; valor nil = variável ausente (unset).
// t.Setenv registra a restauração antes do Unsetenv.
func sec11Env(t *testing.T, env map[string]*string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, "")
		if v == nil {
			require.NoError(t, os.Unsetenv(k))
			continue
		}
		t.Setenv(k, *v)
	}
}

func sp(s string) *string { return &s }

// sec11Base: ambiente válido exceto pelas credenciais do banco (sem as envs
// de refresh, que ficam no padrão).
func sec11Base() map[string]*string {
	return map[string]*string{
		"JWT_SECRET":                    sp("segredo-sec11"),
		"JWT_TTL":                       sp("24h"),
		"PASSWORD_PEPPER":               sp("pepper-de-teste-com-pelo-menos-32-bytes"),
		"REFRESH_REUSE_SUPPRESS_WINDOW": nil,
		"REFRESH_CLEANUP_INTERVAL":      nil,
		"REFRESH_TOKEN_RETENCAO":        nil,
		"ARGON2_MEMORIA_KIB":            nil,
		"ARGON2_ITERACOES":              nil,
		"ARGON2_PARALELISMO":            nil,
	}
}

func TestSEC11_Load_ExigeCredenciaisDB(t *testing.T) {
	casos := []struct {
		nome           string
		usuario, senha *string
	}{
		{"ambos ausentes", nil, nil},
		{"só usuário", sp(sec11Usuario), nil},
		{"só senha", nil, sp(sec11Senha)},
		{"usuário vazio", sp(""), sp(sec11Senha)},
		{"senha vazia", sp(sec11Usuario), sp("")},
		{"usuário só espaços", sp("   \t "), sp(sec11Senha)},
		{"ambos vazios", sp(""), sp("")},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = c.usuario
			env["DB_SENHA"] = c.senha
			sec11Env(t, env)

			cfg, err := config.Load()
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrCredenciaisDB), "err=%v", err)
			assert.Nil(t, cfg)
			assert.NotContains(t, err.Error(), sec11Senha, "a senha nunca vai na mensagem")
			assert.NotContains(t, err.Error(), sec11Usuario, "o usuário não vai na mensagem")
			assert.Contains(t, err.Error(), "defina DB_USUARIO/DB_SENHA no .env")
		})
	}
}

func TestSEC11_Load_ComCredenciais_OK(t *testing.T) {
	casos := []struct {
		nome                   string
		usuario, senha         string
		wantUsuario, wantSenha string
	}{
		{"valores simples", "u", "s", "u", "s"},
		// A senha não é "trimada": espaços podem fazer parte dela.
		{"senha só com espaços é aceita como está", "u", "  ", "u", "  "},
		{"usuário com espaços nas pontas é mantido (checagem usa TrimSpace)", " u ", "s", " u ", "s"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp(c.usuario)
			env["DB_SENHA"] = sp(c.senha)
			sec11Env(t, env)

			cfg, err := config.Load()
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, c.wantUsuario, cfg.DBUsuario)
			assert.Equal(t, c.wantSenha, cfg.DBSenha)
		})
	}
}

// Sem as envs, nada cai no antigo default golang/golang.
func TestSEC11_Load_SemDefaultGolang(t *testing.T) {
	env := sec11Base()
	env["DB_USUARIO"] = nil
	env["DB_SENHA"] = nil
	sec11Env(t, env)

	cfg, err := config.Load()
	require.ErrorIs(t, err, config.ErrCredenciaisDB)
	assert.Nil(t, cfg)
	assert.NotContains(t, err.Error(), "golang")

	// Mesmo o Load sem exigência não inventa credenciais.
	cfg, err = config.LoadSemCredenciaisDB()
	require.NoError(t, err)
	assert.Empty(t, cfg.DBUsuario)
	assert.Empty(t, cfg.DBSenha)
	assert.NotContains(t, cfg.DSN(), "golang")
}

func TestSEC11_LoadSemCredenciaisDB(t *testing.T) {
	casos := []struct {
		nome    string
		env     map[string]*string
		wantErr string
		check   func(t *testing.T, c *config.Config)
	}{
		{
			nome: "passa sem DB_USUARIO/DB_SENHA",
			env:  map[string]*string{"DB_USUARIO": nil, "DB_SENHA": nil},
			check: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "segredo-sec11", c.JWTSecret)
				assert.Equal(t, uint32(65536), c.HashSenha.MemoriaKiB)
			},
		},
		{
			nome: "com credenciais também devolve os valores",
			env:  map[string]*string{"DB_USUARIO": sp("u"), "DB_SENHA": sp("s")},
			check: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "u", c.DBUsuario)
				assert.Equal(t, "s", c.DBSenha)
			},
		},
		{nome: "mantém a validação de JWT_SECRET", env: map[string]*string{"JWT_SECRET": nil}, wantErr: "JWT_SECRET"},
		{nome: "mantém a validação de JWT_TTL", env: map[string]*string{"JWT_TTL": sp("abc")}, wantErr: "JWT_TTL"},
		{nome: "mantém a validação de PASSWORD_PEPPER", env: map[string]*string{"PASSWORD_PEPPER": sp("curto")}, wantErr: "PASSWORD_PEPPER"},
		{nome: "mantém a validação de ARGON2_MEMORIA_KIB", env: map[string]*string{"ARGON2_MEMORIA_KIB": sp("99")}, wantErr: "ARGON2_MEMORIA_KIB"},
		{nome: "mantém a validação de REFRESH_REUSE_SUPPRESS_WINDOW", env: map[string]*string{"REFRESH_REUSE_SUPPRESS_WINDOW": sp("10s")}, wantErr: "REFRESH_REUSE_SUPPRESS_WINDOW"},
		{nome: "mantém a validação de REFRESH_CLEANUP_INTERVAL", env: map[string]*string{"REFRESH_CLEANUP_INTERVAL": sp("30s")}, wantErr: "REFRESH_CLEANUP_INTERVAL"},
		{nome: "mantém a validação de REFRESH_TOKEN_RETENCAO", env: map[string]*string{"REFRESH_TOKEN_RETENCAO": sp("1h")}, wantErr: "REFRESH_TOKEN_RETENCAO"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = nil
			env["DB_SENHA"] = nil
			for k, v := range c.env {
				env[k] = v
			}
			sec11Env(t, env)

			cfg, err := config.LoadSemCredenciaisDB()
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
				assert.False(t, errors.Is(err, config.ErrCredenciaisDB))
				assert.Nil(t, cfg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, cfg)
			c.check(t, cfg)
		})
	}
}

// Ordem: sem DB e com outro erro de configuração, o erro existente vem
// primeiro (a checagem de credenciais fica no fim do Load).
func TestSEC11_Load_OrdemDosErros(t *testing.T) {
	casos := []struct {
		nome    string
		env     map[string]*string
		wantErr string
	}{
		{"sem JWT_SECRET e sem DB → JWT_SECRET", map[string]*string{"JWT_SECRET": nil}, "JWT_SECRET"},
		{"JWT_SECRET vazio e sem DB → JWT_SECRET", map[string]*string{"JWT_SECRET": sp("")}, "JWT_SECRET"},
		{"JWT_TTL inválido e sem DB → JWT_TTL", map[string]*string{"JWT_TTL": sp("xyz")}, "JWT_TTL"},
		{"sem PASSWORD_PEPPER e sem DB → PASSWORD_PEPPER", map[string]*string{"PASSWORD_PEPPER": nil}, "PASSWORD_PEPPER"},
		{"janela de reuso inválida e sem DB → REFRESH_REUSE_SUPPRESS_WINDOW", map[string]*string{"REFRESH_REUSE_SUPPRESS_WINDOW": sp("abc")}, "REFRESH_REUSE_SUPPRESS_WINDOW"},
		{"retenção inválida e sem DB → REFRESH_TOKEN_RETENCAO", map[string]*string{"REFRESH_TOKEN_RETENCAO": sp("1h")}, "REFRESH_TOKEN_RETENCAO"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = nil
			env["DB_SENHA"] = nil
			for k, v := range c.env {
				env[k] = v
			}
			sec11Env(t, env)

			cfg, err := config.Load()
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Contains(t, err.Error(), c.wantErr)
			assert.False(t, errors.Is(err, config.ErrCredenciaisDB), "o erro de credenciais só vem por último")
		})
	}
}
