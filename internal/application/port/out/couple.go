package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
)

// CoupleRepository 는 커플 저장소다.
type CoupleRepository interface {
	// FindActiveByUser 는 사용자의 활성 커플을 찾는다. 없으면 ErrNotFound.
	FindActiveByUser(ctx context.Context, userID uuid.UUID) (couple.Couple, error)
	Create(ctx context.Context, c couple.Couple) error
}

// InviteRepository 는 초대 코드 저장소다.
type InviteRepository interface {
	// RevokeUnused 는 creator 가 만든 미사용·미폐기 코드를 모두 무효화한다.
	RevokeUnused(ctx context.Context, creatorID uuid.UUID, now time.Time) error
	// Create 는 코드를 저장한다. 코드가 겹치면 ErrDuplicate.
	Create(ctx context.Context, inv couple.Invite) error
	// FindByCodeForUpdate 는 코드를 잠그고 읽는다. 없으면 ErrNotFound.
	FindByCodeForUpdate(ctx context.Context, code string) (couple.Invite, error)
	MarkUsed(ctx context.Context, id, usedBy uuid.UUID, now time.Time) error
}
