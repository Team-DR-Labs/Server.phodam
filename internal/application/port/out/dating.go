package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
)

// ThemeHistory 는 커플이 쓴 테마 기록이다.
type ThemeHistory struct {
	Used map[uuid.UUID]bool
	Last *uuid.UUID
}

// ThemeRepository 는 테마·주제 저장소 (마이그레이션 시드) 다.
type ThemeRepository interface {
	ListThemes(ctx context.Context) ([]dating.Theme, error)
	ListTopics(ctx context.Context, themeID uuid.UUID) ([]dating.Topic, error)
}

// DateRepository 는 데이트와 참여자 저장소다.
type DateRepository interface {
	// Create 는 데이트와 참여자를 저장한다. 커플에 진행 중 데이트가 있으면 ErrDuplicate.
	Create(ctx context.Context, d dating.Date) error
	// Get 은 데이트를 참여자·테마·주제와 함께 읽는다. 없으면 ErrNotFound.
	Get(ctx context.Context, id uuid.UUID) (dating.Date, error)
	// GetForUpdate 는 데이트 행을 잠그고 읽는다.
	GetForUpdate(ctx context.Context, id uuid.UUID) (dating.Date, error)
	// FindInProgressByCouple 은 커플의 in_progress 데이트를 찾는다. 없으면 ErrNotFound.
	FindInProgressByCouple(ctx context.Context, coupleID uuid.UUID) (dating.Date, error)
	ThemeHistory(ctx context.Context, coupleID uuid.UUID) (ThemeHistory, error)
	UpdateParticipant(ctx context.Context, dateID uuid.UUID, p dating.Participant) error
	// MarkRevealed 는 in_progress 데이트를 revealed 로 바꾼다.
	MarkRevealed(ctx context.Context, id uuid.UUID, at time.Time) error
	// ListSubmittedByUser 는 userID 가 제출한 데이트를 started_at 내림차순으로 읽는다.
	ListSubmittedByUser(ctx context.Context, userID uuid.UUID, after *diary.Cursor, limit int) ([]dating.Date, error)

	// Expire 는 마감이 지난 in_progress 데이트 하나를 expired 로 바꾼다 (요청 시점 즉시 만료).
	Expire(ctx context.Context, id uuid.UUID, now time.Time) error
	// ExpireDue 는 마감이 지난 in_progress 데이트를 SKIP LOCKED 로 잡아 expired 로 바꾼다.
	ExpireDue(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error)
	// ClaimReminders 는 마감 임박 데이트의 reminded_at 을 기록하고 미제출 참여자를 반환한다.
	ClaimReminders(ctx context.Context, now time.Time, limit int) ([]Reminder, error)
}

// Reminder 는 마감 임박 알림 대상이다.
type Reminder struct {
	DateID  uuid.UUID
	UserIDs []uuid.UUID
}
