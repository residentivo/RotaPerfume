package config_test

// RISCO-01: importar o pacote config (import em branco de shared/tz) fixa
// time.Local em -03:00 sem horário de verão.
// BUG-04: o DSN leva clientFoundRows=true e loc=Local.

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

func TestImportConfig_FixaTimeLocalEmMenos3(t *testing.T) {
	datas := []time.Time{
		time.Now(),
		time.Date(2018, 11, 4, 0, 0, 0, 0, time.Local),   // antigo início do horário de verão
		time.Date(2019, 2, 16, 23, 30, 0, 0, time.Local), // antigo fim do horário de verão
		time.Date(2026, 1, 15, 12, 0, 0, 0, time.Local),  // verão
		time.Date(2026, 7, 15, 12, 0, 0, 0, time.Local),  // inverno
	}
	for _, d := range datas {
		t.Run(d.Format("2006-01-02T15:04"), func(t *testing.T) {
			_, off := d.In(time.Local).Zone()
			assert.Equal(t, -10800, off)
		})
	}

	d, err := time.ParseInLocation("2006-01-02", "2018-11-04", time.Local)
	require.NoError(t, err)
	assert.Equal(t, "2018-11-04T00:00:00-03:00", d.Format(time.RFC3339),
		"meia-noite de 2018-11-04 deve existir no fuso fixo")
}

func TestDSN(t *testing.T) {
	cfg := &config.Config{DBUsuario: "u", DBSenha: "p", DBHost: "h", DBPort: "3306", DBName: "db"}
	dsn := cfg.DSN()

	assert.True(t, strings.HasPrefix(dsn, "u:p@tcp(h:3306)/db?"), dsn)
	for _, param := range []string{
		"parseTime=true",
		"charset=utf8mb4",
		"collation=utf8mb4_unicode_ci",
		"loc=Local",
		"clientFoundRows=true",
	} {
		t.Run(param, func(t *testing.T) {
			assert.Contains(t, dsn, param)
		})
	}
	assert.NotContains(t, dsn, "time_zone", "fuso vem do pacote tz, não do DSN")
}

func TestLoad(t *testing.T) {
	casos := []struct {
		nome    string
		env     map[string]string
		wantErr string
		check   func(t *testing.T, c *config.Config)
	}{
		{
			nome:    "sem JWT_SECRET",
			env:     map[string]string{"JWT_SECRET": ""},
			wantErr: "JWT_SECRET",
		},
		{
			nome:    "JWT_TTL invalido",
			env:     map[string]string{"JWT_SECRET": "s", "JWT_TTL": "abc"},
			wantErr: "JWT_TTL",
		},
		{
			nome:    "BCRYPT_COST nao numerico",
			env:     map[string]string{"JWT_SECRET": "s", "BCRYPT_COST": "x"},
			wantErr: "BCRYPT_COST",
		},
		{
			nome:    "BCRYPT_COST abaixo do minimo",
			env:     map[string]string{"JWT_SECRET": "s", "BCRYPT_COST": "3"},
			wantErr: "BCRYPT_COST",
		},
		{
			nome:    "BCRYPT_COST acima do maximo",
			env:     map[string]string{"JWT_SECRET": "s", "BCRYPT_COST": "32"},
			wantErr: "BCRYPT_COST",
		},
		{
			nome: "defaults",
			env: map[string]string{
				"JWT_SECRET": "s", "JWT_TTL": "", "BCRYPT_COST": "", "DB_HOST": "", "DB_PORT": "",
				"DB_NAME": "", "DB_USUARIO": "", "DB_SENHA": "", "VERBOSE": "", "LOG_LEVEL": "",
				"CORS_ALLOWED_ORIGINS": "", "TRUST_PROXY_HEADERS": "", "JWT_ISSUER": "",
			},
			check: func(t *testing.T, c *config.Config) {
				assert.Equal(t, "localhost", c.DBHost)
				assert.Equal(t, "3306", c.DBPort)
				assert.Equal(t, "rotaperfumes", c.DBName)
				assert.Equal(t, "rotaperfumes", c.JWTIssuer)
				assert.Equal(t, 24*time.Hour, c.JWTTTL)
				assert.Equal(t, 12, c.BCryptCost)
				assert.False(t, c.Verbose)
				assert.False(t, c.TrustProxyHeaders)
				assert.Equal(t, []string{"http://localhost:3000", "http://127.0.0.1:3000"}, c.CORSAllowedOrigins)
			},
		},
		{
			nome: "valores customizados",
			env: map[string]string{
				"JWT_SECRET": "s", "JWT_TTL": "2h", "BCRYPT_COST": "4", "LOG_LEVEL": "debug", "VERBOSE": "",
				"CORS_ALLOWED_ORIGINS": " https://a.com , ,https://b.com", "TRUST_PROXY_HEADERS": "true",
			},
			check: func(t *testing.T, c *config.Config) {
				assert.Equal(t, 2*time.Hour, c.JWTTTL)
				assert.Equal(t, 4, c.BCryptCost)
				assert.True(t, c.Verbose)
				assert.True(t, c.TrustProxyHeaders)
				assert.Equal(t, []string{"http://localhost:3000", "http://127.0.0.1:3000", "https://a.com", "https://b.com"}, c.CORSAllowedOrigins)
			},
		},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			c, err := config.Load()
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				assert.Nil(t, c)
				return
			}
			require.NoError(t, err)
			tc.check(t, c)
		})
	}
}
