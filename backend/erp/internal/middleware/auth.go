package middleware

import (
	"net/http"
	"strings"

	"backend/erp/internal/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"erro": "Token nao fornecido"}`, http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, `{"erro": "Formato de token invalido"}`, http.StatusUnauthorized)
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.AppConfig.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, `{"erro": "Token invalido ou expirado"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func GetPathParams(r *http.Request) map[string]string {
	return mux.Vars(r)
}
