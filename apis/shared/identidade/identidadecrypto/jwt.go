package identidadecrypto

import (
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

// ErrTokenInvalido: assinatura, algoritmo, formato ou validade (exp/nbf)
// recusados. Não distingue o motivo (o legado também não distinguia).
var ErrTokenInvalido = errors.New("identidadecrypto: token inválido")

// Claims é o payload do access token (o mesmo do legado: "uid" e "role").
type Claims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWT emite e valida access tokens HS256 (I-01). O issuer é gravado mas não
// é exigido na validação (SEC-18, GAP preservado).
type JWT struct {
	segredo []byte
	emissor string
	ttl     time.Duration
}

// NovoJWT usa JWT_SECRET, JWT_ISSUER e JWT_TTL da config (nil: panic).
func NovoJWT(cfg *config.Config) *JWT {
	if cfg == nil {
		panic("identidadecrypto: NovoJWT exige config")
	}
	return &JWT{segredo: []byte(cfg.JWTSecret), emissor: cfg.JWTIssuer, ttl: cfg.JWTTTL}
}

// TTL é a validade do access token (vai para o cookie e para expires_in).
func (j *JWT) TTL() time.Duration { return j.ttl }

// Emitir assina um access token com iat/nbf/exp (I-01).
func (j *JWT) Emitir(userID int64, role kernel.Role) (string, error) {
	agora := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.emissor,
			IssuedAt:  jwt.NewNumericDate(agora),
			ExpiresAt: jwt.NewNumericDate(agora.Add(j.ttl)),
			NotBefore: jwt.NewNumericDate(agora),
			Subject:   strconv.FormatInt(userID, 10),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.segredo)
}

// Ler valida o token (só HS256) e devolve todos os claims.
func (j *JWT) Ler(token string) (*Claims, error) {
	claims := &Claims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))
	tok, err := parser.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return j.segredo, nil })
	if err != nil || !tok.Valid {
		return nil, ErrTokenInvalido
	}
	return claims, nil
}

// ValidarAcesso é a porta do httpx.Autenticador: valida e devolve o que a
// autenticação precisa (user_id, role do token, iat e exp).
func (j *JWT) ValidarAcesso(token string) (identidade.ClaimsAcesso, error) {
	c, err := j.Ler(token)
	if err != nil {
		return identidade.ClaimsAcesso{}, err
	}
	return identidade.ClaimsAcesso{
		UserID:    c.UserID,
		Role:      kernel.RoleDoBanco(c.Role),
		EmitidoEm: tempoDe(c.IssuedAt),
		ExpiraEm:  tempoDe(c.ExpiresAt),
	}, nil
}

func tempoDe(d *jwt.NumericDate) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}
