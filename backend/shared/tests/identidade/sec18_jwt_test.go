package identidade_test

// SEC-18 (Lote 24, TestBrain): validação do access token.
//   - Ler/ValidarAcesso recusam (sempre com ErrTokenInvalido, sem distinguir o
//     motivo): issuer errado, sem iss, sem exp, exp passado, nbf futuro, iat
//     futuro, alg=none, HS512 e assinatura com outro segredo. Token válido
//     passa; sem iat passa (o corte de sessão é que recusa, I-04).
//   - NovoJWT dá panic com issuer vazio/só espaços (WithIssuer("") desligaria
//     a checagem) e com segredo abaixo de config.JWTSecretMinBytes; 32 bytes
//     é aceito. A mensagem do panic nunca traz o segredo.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/kernel"
)

const sec18Issuer = "rotaperfumes-sec18"

// sec18Segredo tem exatamente config.JWTSecretMinBytes bytes.
var sec18Segredo = strings.Repeat("k", config.JWTSecretMinBytes)

func sec18Cfg() *config.Config {
	return &config.Config{JWTSecret: sec18Segredo, JWTIssuer: sec18Issuer, JWTTTL: time.Hour}
}

// sec18Assinar assina claims com o método e segredo informados.
func sec18Assinar(t *testing.T, m jwt.SigningMethod, segredo any, rc jwt.RegisteredClaims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(m, identidadecrypto.Claims{UserID: 7, Role: "normal", RegisteredClaims: rc}).SignedString(segredo)
	require.NoError(t, err)
	return tok
}

// sec18Claims: claims válidos (iss, iat, nbf e exp), mutados pelo caso.
func sec18Claims(mutar func(*jwt.RegisteredClaims)) jwt.RegisteredClaims {
	agora := time.Now()
	rc := jwt.RegisteredClaims{
		Issuer:    sec18Issuer,
		IssuedAt:  jwt.NewNumericDate(agora.Add(-time.Minute)),
		NotBefore: jwt.NewNumericDate(agora.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(agora.Add(time.Hour)),
		Subject:   "7",
	}
	if mutar != nil {
		mutar(&rc)
	}
	return rc
}

func TestSEC18_Ler_ValidacaoDeClaims(t *testing.T) {
	segredo := []byte(sec18Segredo)
	casos := []struct {
		nome   string
		token  func(t *testing.T) string
		valido bool
	}{
		{"válido (iss, iat, nbf, exp)", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(nil))
		}, true},
		{"sem iat é aceito", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) { rc.IssuedAt = nil }))
		}, true},
		{"sem nbf é aceito", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) { rc.NotBefore = nil }))
		}, true},
		{"issuer errado", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) { rc.Issuer = "outro-emissor" }))
		}, false},
		{"issuer com espaço a mais", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) { rc.Issuer = sec18Issuer + " " }))
		}, false},
		{"issuer em maiúsculas", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) { rc.Issuer = strings.ToUpper(sec18Issuer) }))
		}, false},
		{"sem iss", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) { rc.Issuer = "" }))
		}, false},
		{"sem exp", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) { rc.ExpiresAt = nil }))
		}, false},
		{"exp no passado", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) {
				rc.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Second))
			}))
		}, false},
		{"nbf no futuro", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) {
				rc.NotBefore = jwt.NewNumericDate(time.Now().Add(time.Minute))
			}))
		}, false},
		{"iat no futuro", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, segredo, sec18Claims(func(rc *jwt.RegisteredClaims) {
				rc.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Minute))
			}))
		}, false},
		{"alg=none", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, sec18Claims(nil))
		}, false},
		{"HS512 com o mesmo segredo", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS512, segredo, sec18Claims(nil))
		}, false},
		{"HS384 com o mesmo segredo", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS384, segredo, sec18Claims(nil))
		}, false},
		{"assinado com outro segredo", func(t *testing.T) string {
			return sec18Assinar(t, jwt.SigningMethodHS256, []byte(strings.Repeat("z", 32)), sec18Claims(nil))
		}, false},
		{"emitido por JWT com outro issuer e o mesmo segredo", func(t *testing.T) string {
			cfg := sec18Cfg()
			cfg.JWTIssuer = "outro-emissor"
			tok, err := identidadecrypto.NovoJWT(cfg).Emitir(7, kernel.RoleNormal)
			require.NoError(t, err)
			return tok
		}, false},
		{"string vazia", func(*testing.T) string { return "" }, false},
	}
	j := identidadecrypto.NovoJWT(sec18Cfg())
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			tok := c.token(t)

			claims, errLer := j.Ler(tok)
			acesso, errAcesso := j.ValidarAcesso(tok)
			if c.valido {
				require.NoError(t, errLer)
				require.NoError(t, errAcesso)
				assert.Equal(t, int64(7), claims.UserID)
				assert.Equal(t, int64(7), acesso.UserID)
				assert.Equal(t, kernel.RoleNormal, acesso.Role)
				require.NotNil(t, acesso.ExpiraEm)
				return
			}
			assert.Nil(t, claims)
			assert.Equal(t, identidadecrypto.ErrTokenInvalido, errLer, "Ler devolve só o sentinela, sem o motivo")
			assert.Equal(t, identidadecrypto.ErrTokenInvalido, errAcesso, "ValidarAcesso devolve só o sentinela")
			assert.Zero(t, acesso.UserID)
		})
	}
}

// O token emitido pelo próprio JWT carrega iss/iat/nbf/exp e é aceito.
func TestSEC18_Emitir_TemIssuerEExp(t *testing.T) {
	j := identidadecrypto.NovoJWT(sec18Cfg())
	tok, err := j.Emitir(3, kernel.RoleAdmin)
	require.NoError(t, err)
	c, err := j.Ler(tok)
	require.NoError(t, err)
	assert.Equal(t, sec18Issuer, c.Issuer)
	require.NotNil(t, c.ExpiresAt)
	require.NotNil(t, c.IssuedAt)
	assert.WithinDuration(t, time.Now().Add(time.Hour), c.ExpiresAt.Time, 5*time.Second)
}

func TestSEC18_NovoJWT_Montagem(t *testing.T) {
	casos := []struct {
		nome    string
		segredo string
		issuer  string
		panica  bool
	}{
		{"issuer vazio", sec18Segredo, "", true},
		{"issuer só espaços", sec18Segredo, "   ", true},
		{"issuer só tab/quebra", sec18Segredo, "\t\n", true},
		{"segredo vazio", "", sec18Issuer, true},
		{"segredo de 1 byte", "s", sec18Issuer, true},
		{"segredo de 31 bytes", strings.Repeat("a", config.JWTSecretMinBytes-1), sec18Issuer, true},
		{"segredo de 32 bytes", strings.Repeat("a", config.JWTSecretMinBytes), sec18Issuer, false},
		{"segredo de 64 bytes", strings.Repeat("a", 64), sec18Issuer, false},
		// O mínimo é em bytes, não em runas ("ã" tem 2 bytes).
		{"segredo multibyte de 32 bytes (16 runas)", strings.Repeat("ã", 16), sec18Issuer, false},
		{"segredo multibyte de 31 bytes (16 runas)", strings.Repeat("ã", 15) + "a", sec18Issuer, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			cfg := &config.Config{JWTSecret: c.segredo, JWTIssuer: c.issuer, JWTTTL: time.Hour}
			if !c.panica {
				assert.NotPanics(t, func() { identidadecrypto.NovoJWT(cfg) })
				return
			}
			var recuperado any
			func() {
				defer func() { recuperado = recover() }()
				identidadecrypto.NovoJWT(cfg)
			}()
			require.NotNil(t, recuperado, "esperava panic na montagem")
			if c.segredo != "" {
				assert.NotContains(t, fmt.Sprint(recuperado), c.segredo, "o panic nunca traz o segredo")
			}
		})
	}
	assert.Equal(t, 32, config.JWTSecretMinBytes)
}
