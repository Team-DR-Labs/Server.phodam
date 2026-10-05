package in

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// CoupleView 는 요청자 시점의 커플 정보다.
type CoupleView struct {
	ID          uuid.UUID
	Partner     user.Profile
	ConnectedAt time.Time
}

// Me 는 내 상태 요약이다.
type Me struct {
	User        user.Profile
	FilmBalance int
	Couple      *CoupleView
	CurrentDate *dating.View
}

// MeUseCase 는 내 정보 유스케이스다.
type MeUseCase interface {
	Get(ctx context.Context, userID uuid.UUID) (Me, error)
	UpdateNickname(ctx context.Context, userID uuid.UUID, nickname string) (Me, error)
	RegisterDevice(ctx context.Context, userID uuid.UUID, token, platform string) error
}

// Invite 는 발급된 초대 코드다.
type Invite struct {
	Code      string
	ExpiresAt time.Time
}

// CoupleUseCase 는 커플 연결 유스케이스다.
type CoupleUseCase interface {
	Get(ctx context.Context, userID uuid.UUID) (CoupleView, error)
	CreateInvite(ctx context.Context, userID uuid.UUID) (Invite, error)
	Join(ctx context.Context, userID uuid.UUID, code string) (CoupleView, error)
}

// AdminUseCase 는 관리자 유스케이스다.
type AdminUseCase interface {
	GrantFilm(ctx context.Context, userID uuid.UUID, shots int, reason *string) (balance int, err error)
}
