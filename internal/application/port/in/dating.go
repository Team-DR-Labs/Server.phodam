package in

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
)

// DateUseCase 는 데이트 시작·참여·제출 유스케이스다.
type DateUseCase interface {
	Start(ctx context.Context, userID uuid.UUID) (dating.View, error)
	// Current 는 진행 중 데이트를 반환한다. 없으면 nil.
	Current(ctx context.Context, userID uuid.UUID) (*dating.View, error)
	Get(ctx context.Context, userID, dateID uuid.UUID) (dating.View, error)
	Join(ctx context.Context, userID, dateID uuid.UUID) (dating.View, error)
	Submit(ctx context.Context, userID, dateID, photoID uuid.UUID, caption *string) (dating.View, error)
}

// UploadTarget 은 presigned PUT 업로드 정보다.
type UploadTarget struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

// ShotReservation 은 샷 예약 결과다.
type ShotReservation struct {
	Photo       photo.Photo
	Upload      UploadTarget
	FilmBalance int
}

// PhotoWithURL 은 presigned GET URL 이 붙은 사진이다.
type PhotoWithURL struct {
	Photo        photo.Photo
	URL          string
	URLExpiresAt time.Time
}

// Receivable 은 수령 가능한 사진 목록이다.
type Receivable struct {
	ReceiveDeadlineAt time.Time
	Items             []PhotoWithURL
}

// PhotoUseCase 는 촬영·업로드·수령 유스케이스다.
type PhotoUseCase interface {
	ReserveShot(ctx context.Context, userID, dateID uuid.UUID) (ShotReservation, error)
	ReissueUploadURL(ctx context.Context, userID, photoID uuid.UUID) (UploadTarget, error)
	CompleteUpload(ctx context.Context, userID, photoID uuid.UUID) (photo.Photo, error)
	ListMine(ctx context.Context, userID, dateID uuid.UUID) ([]PhotoWithURL, error)
	Receivable(ctx context.Context, userID, dateID uuid.UUID) (Receivable, error)
	AckReceived(ctx context.Context, userID, photoID uuid.UUID) (photo.Photo, error)
}

// DiaryUseCase 는 일기 열람 유스케이스다.
type DiaryUseCase interface {
	List(ctx context.Context, userID uuid.UUID, cursor string, limit int) (diary.Page, error)
	Get(ctx context.Context, userID, dateID uuid.UUID) (diary.Detail, error)
}

// MaintenanceUseCase 는 주기 작업(정책 §11) 유스케이스다.
type MaintenanceUseCase interface {
	RunOnce(ctx context.Context)
}
