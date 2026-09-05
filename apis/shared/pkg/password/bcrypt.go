// Package password provides secure password hashing using bcrypt.
package password

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plain text password using bcrypt.
func HashPassword(plainPassword string, cost int) (string, error) {
	if cost < 4 {
		cost = 4
	}
	if cost > 31 {
		cost = 31
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(plainPassword), cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(bytes), nil
}

// CheckPassword compares a hashed password with a plain text password.
func CheckPassword(hashedPassword, plainPassword string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
	if err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}
	return nil
}

// HashCost generates a bcrypt hash cost from a cost string (for use in scripts).
func HashCost(plainPassword string, cost int) string {
	hash, err := HashPassword(plainPassword, cost)
	if err != nil {
		return ""
	}
	return hash
}
