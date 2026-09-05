package jwt

import (
	"os"
	"testing"
	"time"

	"rotaperfumes/shared/pkg/config"
)

func testConfig() *config.Config {
	return &config.Config{
		JWTSecret: "this-is-a-test-secret-key-32-chars-min",
		JWTTTL:    "1h",
		JWTIssuer: "rotaperfumes-test",
	}
}

func TestJWTService_NewJWTService(t *testing.T) {
	cfg := testConfig()

	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     cfg,
			wantErr: false,
		},
		{
			name: "secret too short",
			cfg: &config.Config{
				JWTSecret: "short",
				JWTTTL:    "1h",
				JWTIssuer: "test",
			},
			wantErr: true,
		},
		{
			name: "invalid duration",
			cfg: &config.Config{
				JWTSecret: "this-is-a-test-secret-key-32-chars-min",
				JWTTTL:    "invalid",
				JWTIssuer: "test",
			},
			wantErr: false, // duration error falls back to 24h
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewJWTService(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewJWTService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && svc == nil {
				t.Error("NewJWTService() returned nil service")
			}
		})
	}
}

func TestJWTService_GenerateAndValidate(t *testing.T) {
	cfg := testConfig()
	svc, err := NewJWTService(cfg)
	if err != nil {
		t.Fatalf("NewJWTService() failed: %v", err)
	}

	tests := []struct {
		name   string
		userID int64
		login  string
		role   string
	}{
		{
			name:   "admin user",
			userID: 1,
			login:  "admin",
			role:   "RH",
		},
		{
			name:   "vendedor user",
			userID: 999,
			login:  "vendedor.teste",
			role:   "VENDEDOR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, expiresAt, err := svc.GenerateToken(tt.userID, tt.login, tt.role)
			if err != nil {
				t.Fatalf("GenerateToken() error = %v", err)
			}
			if token == "" {
				t.Error("GenerateToken() returned empty token")
			}
			if expiresAt.Before(time.Now()) {
				t.Error("GenerateToken() returned past expiration time")
			}

			// Validate the token
			claims, err := svc.ValidateToken(token)
			if err != nil {
				t.Fatalf("ValidateToken() error = %v", err)
			}
			if claims.UserID != tt.userID {
				t.Errorf("ValidateToken() UserID = %v, want %v", claims.UserID, tt.userID)
			}
			if claims.Login != tt.login {
				t.Errorf("ValidateToken() Login = %v, want %v", claims.Login, tt.login)
			}
			if claims.Role != tt.role {
				t.Errorf("ValidateToken() Role = %v, want %v", claims.Role, tt.role)
			}
			if claims.Issuer != cfg.JWTIssuer {
				t.Errorf("ValidateToken() Issuer = %v, want %v", claims.Issuer, cfg.JWTIssuer)
			}
		})
	}
}

func TestJWTService_ValidateToken_Errors(t *testing.T) {
	cfg := testConfig()
	svc, err := NewJWTService(cfg)
	if err != nil {
		t.Fatalf("NewJWTService() failed: %v", err)
	}

	tests := []struct {
		name       string
		token      string
		wantErrStr string
	}{
		{
			name:       "empty token",
			token:      "",
			wantErrStr: "failed to parse token",
		},
		{
			name:       "random string",
			token:      "random.invalid.token",
			wantErrStr: "failed to parse token",
		},
		{
			name:       "malformed JWT",
			token:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature",
			wantErrStr: "failed to parse token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ValidateToken(tt.token)
			if err == nil {
				t.Error("ValidateToken() expected error, got nil")
			}
		})
	}
}

func TestJWTService_DifferentSecret(t *testing.T) {
	cfg1 := testConfig()
	cfg2 := &config.Config{
		JWTSecret: "different-secret-key-32-chars-minimum",
		JWTTTL:    "1h",
		JWTIssuer: "test",
	}

	svc1, _ := NewJWTService(cfg1)
	svc2, _ := NewJWTService(cfg2)

	token, _, _ := svc1.GenerateToken(1, "admin", "RH")
	_, err := svc2.ValidateToken(token)
	if err == nil {
		t.Error("ValidateToken() should fail with different secret")
	}
}

func TestMain(m *testing.M) {
	// Ensure JWT_SECRET env var is not interfering
	os.Unsetenv("JWT_SECRET")
	os.Exit(m.Run())
}
