package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWTMiddleware cria um middleware de autenticação JWT
func JWTMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"erro": "token não fornecido"}`, http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, `{"erro": "formato de token inválido"}`, http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})

			if err != nil || !token.Valid {
				http.Error(w, `{"erro": "token inválido ou expirado"}`, http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"erro": "claims inválidos"}`, http.StatusUnauthorized)
				return
			}

			// Armazena informações do usuário no contexto
			r.Header.Set("X-Usuario-ID", getStringClaim(claims, "id"))
			r.Header.Set("X-Usuario-Login", getStringClaim(claims, "login"))
			r.Header.Set("X-Usuario-Tipo", getStringClaim(claims, "tipo_usuario"))

			next.ServeHTTP(w, r)
		})
	}
}

func getStringClaim(claims jwt.MapClaims, key string) string {
	if val, ok := claims[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// ExtractClaims extrai claims do token
func ExtractClaims(r *http.Request) (map[string]string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, nil
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return nil, nil
	}

	// O middleware já validou o token, pegamos do header
	return map[string]string{
		"id":          r.Header.Get("X-Usuario-ID"),
		"login":       r.Header.Get("X-Usuario-Login"),
		"tipo_usuario": r.Header.Get("X-Usuario-Tipo"),
	}, nil
}
