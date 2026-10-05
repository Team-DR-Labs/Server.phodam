package in

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// AuthResult 는 로그인·갱신 결과다.
type AuthResult struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
	IsNewUser             bool
	User                  user.Profile
}

// AuthUseCase 는 로그인과 토큰 관리 유스케이스다.
type AuthUseCase interface {
	LoginApple(ctx context.Context, identityToken string, nickname *string) (AuthResult, error)
	LoginGoogle(ctx context.Context, idToken string) (AuthResult, error)
	LoginDev(ctx context.Context, devID string, nickname *string) (AuthResult, error)
	Refresh(ctx context.Context, refreshToken string) (AuthResult, error)
	Logout(ctx context.Context, userID uuid.UUID, refreshToken string) error
	// Authenticate 는 액세스 토큰을 검증해 사용자 ID 를 반환한다.
	Authenticate(ctx context.Context, accessToken string) (uuid.UUID, error)
}
