package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
)

// PhotoRepository 는 사진 레코드 저장소다.
type PhotoRepository interface {
	Create(ctx context.Context, p photo.Photo) error
	// Get 은 사진을 읽는다. 없으면 ErrNotFound.
	Get(ctx context.Context, id uuid.UUID) (photo.Photo, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (photo.Photo, error)
	// Update 는 상태 관련 필드를 저장한다.
	Update(ctx context.Context, p photo.Photo) error
	ListByOwner(ctx context.Context, dateID, ownerID uuid.UUID, statuses ...photo.Status) ([]photo.Photo, error)
	CountByOwner(ctx context.Context, dateID, ownerID uuid.UUID) (int, error)
	// DeleteReserved 는 이 데이트에서 owner 의 reserved 사진을 deleted 로 바꾸고 바뀐 사진을 반환한다.
	DeleteReserved(ctx context.Context, dateID, ownerID uuid.UUID, now time.Time) ([]photo.Photo, error)
	MarkTempPurged(ctx context.Context, id uuid.UUID, now time.Time) error

	// 워커용: 각 메서드는 FOR UPDATE SKIP LOCKED 로 대상을 잠근다 (트랜잭션 안에서 호출).
	LockExpiredLeftovers(ctx context.Context, limit int) ([]photo.Photo, error)
	LockReceiveOverdue(ctx context.Context, now time.Time, limit int) ([]photo.Photo, error)
	LockArchivedWithTemp(ctx context.Context, limit int) ([]photo.Photo, error)
}
