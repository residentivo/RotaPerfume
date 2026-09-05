package password

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		cost     int
		wantErr  bool
	}{
		{
			name:     "hash password with cost 10",
			password: "Temp@2026!",
			cost:     10,
			wantErr:  false,
		},
		{
			name:     "hash password with cost 4 (minimum)",
			password: "simple",
			cost:     4,
			wantErr:  false,
		},
		{
			name:     "hash empty password",
			password: "",
			cost:     10,
			wantErr:  false,
		},
		{
			name:     "hash password with unicode",
			password: "senha123áéíóú",
			cost:     10,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password, tt.cost)
			if (err != nil) != tt.wantErr {
				t.Errorf("HashPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if hash == "" {
					t.Error("HashPassword() returned empty hash")
				}
				if hash == tt.password {
					t.Error("HashPassword() returned plain text instead of hash")
				}
				// Hash should start with bcrypt prefix
				if len(hash) < 60 {
					t.Errorf("HashPassword() hash too short: %d chars", len(hash))
				}
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	password := "Temp@2026!"
	hash, err := HashPassword(password, 10)
	if err != nil {
		t.Fatalf("HashPassword() failed: %v", err)
	}

	tests := []struct {
		name     string
		hash     string
		password string
		wantErr  bool
	}{
		{
			name:     "correct password",
			hash:     hash,
			password: password,
			wantErr:  false,
		},
		{
			name:     "wrong password",
			hash:     hash,
			password: "WrongPassword!",
			wantErr:  true,
		},
		{
			name:     "empty password",
			hash:     hash,
			password: "",
			wantErr:  true,
		},
		{
			name:     "invalid hash",
			hash:     "not-a-valid-bcrypt-hash",
			password: "any",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPassword(tt.hash, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHashPasswordDeterminism(t *testing.T) {
	// bcrypt generates different salts each time, so same password
	// should produce different hashes (both valid)
	hash1, err := HashPassword("Test123", 10)
	if err != nil {
		t.Fatalf("HashPassword() first call failed: %v", err)
	}

	hash2, err := HashPassword("Test123", 10)
	if err != nil {
		t.Fatalf("HashPassword() second call failed: %v", err)
	}

	// Hashes should be different due to random salt
	if hash1 == hash2 {
		t.Error("HashPassword() returned same hash for same input - bcrypt should use random salt")
	}

	// But both should verify correctly
	if err := CheckPassword(hash1, "Test123"); err != nil {
		t.Errorf("CheckPassword() failed for hash1: %v", err)
	}
	if err := CheckPassword(hash2, "Test123"); err != nil {
		t.Errorf("CheckPassword() failed for hash2: %v", err)
	}
}
