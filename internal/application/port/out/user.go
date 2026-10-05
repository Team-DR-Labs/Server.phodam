package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// UserRepository 는 사용자 저장소다.
type UserRepository interface {
	Create(ctx context.Context, u user.User) error
	Get(ctx context.Context, id uuid.UUID) (user.User, error)
	// LockForUpdate 는 사용자 행들을 id 순서로 잠근다 (커플 연결 경쟁 방지).
	LockForUpdate(ctx context.Context, ids ...uuid.UUID) error
	UpdateNickname(ctx context.Context, id uuid.UUID, nickname string) error
	// DecrementFilm 은 잔액이 0 보다 클 때만 1 차감한다. 차감하지 못하면 ok=false.
	DecrementFilm(ctx context.Context, id uuid.UUID) (balance int, ok bool, err error)
	// AddFilm 은 잔액에 n 을 더한다. 사용자가 없으면 ErrNotFound.
	AddFilm(ctx context.Context, id uuid.UUID, n int) (balance int, err error)
}

// IdentityRepository 는 외부 로그인 식별자 저장소다.
type IdentityRepository interface {
	// FindUserID 는 (provider, subject) 의 사용자 ID 를 찾는다. 없으면 ErrNotFound.
	FindUserID(ctx context.Context, provider user.Provider, subject string) (uuid.UUID, error)
	// Create 는 식별자를 저장한다. 이미 있으면 ErrDuplicate.
	Create(ctx context.Context, identity user.Identity) error
}

// FilmLedgerRepository 는 필름 원장이다 (추가만 한다).
type FilmLedgerRepository interface {
	Append(ctx context.Context, entry user.LedgerEntry) error
}

// RefreshToken 은 저장된 리프레시 토큰 (해시만 보관) 이다.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// RefreshTokenRepository 는 리프레시 토큰 저장소다.
type RefreshTokenRepository interface {
	Create(ctx context.Context, t RefreshToken) error
	// Consume 은 유효한(미폐기, 미만료) 토큰을 폐기하고 소유자를 반환한다. 없으면 ErrNotFound.
	Consume(ctx context.Context, tokenHash string, now time.Time) (uuid.UUID, error)
	// Revoke 는 userID 소유 토큰을 폐기한다. 없거나 이미 폐기됐어도 에러가 아니다.
	Revoke(ctx context.Context, userID uuid.UUID, tokenHash string, now time.Time) error
}

// DeviceRepository 는 FCM 기기 토큰 저장소다.
type DeviceRepository interface {
	// Upsert 는 토큰 소유자를 d.UserID 로 등록하거나 옮긴다.
	Upsert(ctx context.Context, d user.Device) error
	ListTokens(ctx context.Context, userIDs ...uuid.UUID) ([]string, error)
	DeleteTokens(ctx context.Context, tokens ...string) error
}
