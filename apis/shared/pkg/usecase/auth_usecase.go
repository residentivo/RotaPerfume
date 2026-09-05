// Package usecase implements application business logic.
package usecase

import (
	"context"
	"errors"
	"time"

	"rotaperfumes/shared/pkg/dto"
	"rotaperfumes/shared/pkg/jwt"
	"rotaperfumes/shared/pkg/logger"
	"rotaperfumes/shared/pkg/password"
	"rotaperfumes/shared/pkg/repository"
)

// Common errors.
var (
	ErrInvalidCredentials = errors.New("invalid login or password")
	ErrUserInactive       = errors.New("user account is inactive")
	ErrUserNotFound       = errors.New("user not found")
)

// AuthUseCase handles authentication business logic.
type AuthUseCase struct {
	userRepo  repository.UserRepository
	jwtSvc   *jwt.JWTService
	bcryptCost int
	log      *logger.Logger
}

// NewAuthUseCase creates a new authentication use case.
func NewAuthUseCase(
	userRepo repository.UserRepository,
	jwtSvc *jwt.JWTService,
	bcryptCost int,
) *AuthUseCase {
	return &AuthUseCase{
		userRepo:   userRepo,
		jwtSvc:     jwtSvc,
		bcryptCost: bcryptCost,
		log:        logger.Get(),
	}
}

// Authenticate validates user credentials and returns a JWT token.
func (uc *AuthUseCase) Authenticate(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	log := uc.log.With("operation", "authenticate", "login", req.Login)

	// Step 1: Log verbose debug
	log.DebugVerbose("Step 1: Looking up user by login")

	// Step 2: Find user by login
	user, err := uc.userRepo.FindByLogin(ctx, req.Login)
	if err != nil {
		log.Error("Failed to query user", "error", err)
		return nil, err
	}

	// Step 3: Check if user exists
	if user == nil {
		log.DebugVerbose("Step 3: User not found in database")
		log.Warn("Authentication failed: user not found", "login", req.Login)
		return nil, ErrInvalidCredentials
	}

	log.DebugVerbose("Step 3: User found", "user_id", user.ID, "role", user.Role)

	// Step 4: Check if user is active
	if !user.IsActive() {
		log.DebugVerbose("Step 4: User account is inactive")
		log.Warn("Authentication failed: user inactive", "login", req.Login, "user_id", user.ID)
		return nil, ErrUserInactive
	}

	log.DebugVerbose("Step 4: User is active")

	// Step 5: Verify password
	log.DebugVerbose("Step 5: Verifying password with bcrypt")
	if err := password.CheckPassword(user.PasswordHash, req.Senha); err != nil {
		log.DebugVerbose("Step 5: Password verification failed")
		log.Warn("Authentication failed: invalid password", "login", req.Login, "user_id", user.ID)
		return nil, ErrInvalidCredentials
	}

	log.DebugVerbose("Step 5: Password verified successfully")

	// Step 6: Generate JWT token
	log.DebugVerbose("Step 6: Generating JWT token")
	token, expiresAt, err := uc.jwtSvc.GenerateToken(user.ID, user.Login, string(user.Role))
	if err != nil {
		log.Error("Failed to generate JWT token", "error", err)
		return nil, err
	}

	log.Info("User authenticated successfully", "user_id", user.ID, "login", user.Login, "role", user.Role)

	return &dto.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		TokenType: "Bearer",
		User: dto.UserInfo{
			ID:    user.ID,
			Login: user.Login,
			Role:  string(user.Role),
		},
	}, nil
}

// GenerateTestToken generates a test token (for development only).
func (uc *AuthUseCase) GenerateTestToken(userID int64, login, role string) (string, time.Time, error) {
	return uc.jwtSvc.GenerateToken(userID, login, role)
}
