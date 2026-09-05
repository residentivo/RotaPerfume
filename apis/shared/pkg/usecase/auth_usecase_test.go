package usecase

import (
	"context"
	"errors"
	"testing"

	"rotaperfumes/shared/pkg/dto"
	"rotaperfumes/shared/pkg/entities"
	"rotaperfumes/shared/pkg/config"
	"rotaperfumes/shared/pkg/jwt"
	"rotaperfumes/shared/pkg/logger"
	"rotaperfumes/shared/pkg/password"
)

// MockUserRepository is a mock implementation of UserRepository for testing.
type MockUserRepository struct {
	FindByLoginFunc func(ctx context.Context, login string) (*entities.User, error)
	FindByIDFunc    func(ctx context.Context, id int64) (*entities.User, error)
}

func (m *MockUserRepository) FindByLogin(ctx context.Context, login string) (*entities.User, error) {
	if m.FindByLoginFunc != nil {
		return m.FindByLoginFunc(ctx, login)
	}
	return nil, nil
}

func (m *MockUserRepository) FindByID(ctx context.Context, id int64) (*entities.User, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, nil
}

func setupTestUseCase(mockRepo *MockUserRepository) *AuthUseCase {
	// Initialize logger (silently)
	logger.Init("error", "text")

	cfg := &config.Config{
		JWTSecret: "test-secret-key-32-chars-minimum-for-testing",
		JWTTTL:    "1h",
		JWTIssuer: "test",
	}
	jwtSvc, _ := jwt.NewJWTService(cfg)

	return NewAuthUseCase(mockRepo, jwtSvc, 10)
}

func loginRequest(login, senha string) *dto.LoginRequest {
	return &dto.LoginRequest{Login: login, Senha: senha}
}

func TestAuthUseCase_Authenticate_Success(t *testing.T) {
	mockRepo := &MockUserRepository{}
	uc := setupTestUseCase(mockRepo)

	correctPassword := "Temp@2026!"
	hash, _ := password.HashPassword(correctPassword, 10)

	mockRepo.FindByLoginFunc = func(ctx context.Context, login string) (*entities.User, error) {
		if login == "admin" {
			return &entities.User{
				ID:           1,
				Login:        "admin",
				PasswordHash: hash,
				Role:         entities.RoleRH,
				Ativo:        true,
			}, nil
		}
		return nil, nil
	}

	resp, err := uc.Authenticate(context.Background(), loginRequest("admin", correctPassword))
	if err != nil {
		t.Fatalf("Authenticate() error = %v, want nil", err)
	}
	if resp == nil {
		t.Fatal("Authenticate() returned nil response")
	}
	if resp.Token == "" {
		t.Error("Authenticate() returned empty token")
	}
	if resp.User.Login != "admin" {
		t.Errorf("Authenticate() User.Login = %v, want admin", resp.User.Login)
	}
	if resp.User.Role != "RH" {
		t.Errorf("Authenticate() User.Role = %v, want RH", resp.User.Role)
	}
}

func TestAuthUseCase_Authenticate_UserNotFound(t *testing.T) {
	mockRepo := &MockUserRepository{}
	uc := setupTestUseCase(mockRepo)

	mockRepo.FindByLoginFunc = func(ctx context.Context, login string) (*entities.User, error) {
		return nil, nil
	}

	_, err := uc.Authenticate(context.Background(), loginRequest("unknown", "pass"))
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestAuthUseCase_Authenticate_WrongPassword(t *testing.T) {
	mockRepo := &MockUserRepository{}
	uc := setupTestUseCase(mockRepo)

	hash, _ := password.HashPassword("correct", 10)

	mockRepo.FindByLoginFunc = func(ctx context.Context, login string) (*entities.User, error) {
		return &entities.User{
			ID:           1,
			Login:        "admin",
			PasswordHash: hash,
			Role:         entities.RoleRH,
			Ativo:        true,
		}, nil
	}

	_, err := uc.Authenticate(context.Background(), loginRequest("admin", "wrongpassword"))
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestAuthUseCase_Authenticate_InactiveUser(t *testing.T) {
	mockRepo := &MockUserRepository{}
	uc := setupTestUseCase(mockRepo)

	hash, _ := password.HashPassword("Temp@2026!", 10)

	mockRepo.FindByLoginFunc = func(ctx context.Context, login string) (*entities.User, error) {
		return &entities.User{
			ID:           1,
			Login:        "admin",
			PasswordHash: hash,
			Role:         entities.RoleRH,
			Ativo:        false,
		}, nil
	}

	_, err := uc.Authenticate(context.Background(), loginRequest("admin", "Temp@2026!"))
	if !errors.Is(err, ErrUserInactive) {
		t.Errorf("Authenticate() error = %v, want %v", err, ErrUserInactive)
	}
}

func TestAuthUseCase_Authenticate_DatabaseError(t *testing.T) {
	mockRepo := &MockUserRepository{}
	uc := setupTestUseCase(mockRepo)

	mockRepo.FindByLoginFunc = func(ctx context.Context, login string) (*entities.User, error) {
		return nil, errors.New("database connection error")
	}

	_, err := uc.Authenticate(context.Background(), loginRequest("admin", "pass"))
	if err == nil {
		t.Error("Authenticate() expected error, got nil")
	}
}
