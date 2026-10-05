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
	// MarkDeleted 는 사진이 아직 from 상태 중 하나일 때만 deleted 로 바꾼다. 바꿨으면 true.
	MarkDeleted(ctx context.Context, id uuid.UUID, now time.Time, from ...photo.Status) (bool, error)

	// 워커 정리 대상 조회. 객체 삭제(네트워크)를 DB 잠금 밖에서 하도록 잠그지 않고 읽는다.
	ListExpiredLeftovers(ctx context.Context, limit int) ([]photo.Photo, error)
	ListReceiveOverdue(ctx context.Context, now time.Time, limit int) ([]photo.Photo, error)
	ListArchivedWithTemp(ctx context.Context, limit int) ([]photo.Photo, error)
}
