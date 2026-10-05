package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
)

type themeModel struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	Title string
}

func (themeModel) TableName() string { return "themes" }

type topicModel struct {
	ID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	ThemeID uuid.UUID `gorm:"type:uuid"`
	Title   string
}

func (topicModel) TableName() string { return "topics" }

type dateModel struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	CoupleID   uuid.UUID `gorm:"type:uuid"`
	ThemeID    uuid.UUID `gorm:"type:uuid"`
	Status     string
	StartedBy  uuid.UUID `gorm:"type:uuid"`
	StartedAt  time.Time
	DeadlineAt time.Time
	RevealedAt *time.Time
	ExpiredAt  *time.Time
	RemindedAt *time.Time
}

func (dateModel) TableName() string { return "dates" }

// dateRow 는 dates 와 테마 제목을 함께 읽는 조회 결과다.
// gorm Raw Scan 은 임베디드 구조체를 채우지 않으므로 필드를 펼쳐 둔다.
type dateRow struct {
	ID         uuid.UUID `gorm:"type:uuid"`
	CoupleID   uuid.UUID `gorm:"type:uuid"`
	ThemeID    uuid.UUID `gorm:"type:uuid"`
	Status     string
	StartedBy  uuid.UUID `gorm:"type:uuid"`
	StartedAt  time.Time
	DeadlineAt time.Time
	RevealedAt *time.Time
	ExpiredAt  *time.Time
	RemindedAt *time.Time
	ThemeTitle string
}

type participantModel struct {
	DateID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	TopicID               uuid.UUID `gorm:"type:uuid"`
	Status                string
	JoinedAt              *time.Time
	SubmittedAt           *time.Time
	ReceiveDeadlineAt     *time.Time
	RepresentativePhotoID *uuid.UUID `gorm:"type:uuid"`
	Caption               *string
}

func (participantModel) TableName() string { return "date_participants" }

// participantRow 는 참여자와 주제 제목을 함께 읽는 조회 결과다.
type participantRow struct {
	DateID                uuid.UUID `gorm:"type:uuid"`
	UserID                uuid.UUID `gorm:"type:uuid"`
	TopicID               uuid.UUID `gorm:"type:uuid"`
	Status                string
	JoinedAt              *time.Time
	SubmittedAt           *time.Time
	ReceiveDeadlineAt     *time.Time
	RepresentativePhotoID *uuid.UUID `gorm:"type:uuid"`
	Caption               *string
	TopicTitle            string
}

func (r participantRow) toDomain() dating.Participant {
	return dating.Participant{
		UserID:                r.UserID,
		Topic:                 dating.Topic{ID: r.TopicID, Title: r.TopicTitle},
		Status:                dating.ParticipantStatus(r.Status),
		JoinedAt:              utcPtr(r.JoinedAt),
		SubmittedAt:           utcPtr(r.SubmittedAt),
		ReceiveDeadlineAt:     utcPtr(r.ReceiveDeadlineAt),
		RepresentativePhotoID: r.RepresentativePhotoID,
		Caption:               r.Caption,
	}
}

func (r dateRow) toDomain(parts []dating.Participant) dating.Date {
	return dating.Date{
		ID:           r.ID,
		CoupleID:     r.CoupleID,
		Theme:        dating.Theme{ID: r.ThemeID, Title: r.ThemeTitle},
		Status:       dating.Status(r.Status),
		StartedBy:    r.StartedBy,
		StartedAt:    r.StartedAt.UTC(),
		DeadlineAt:   r.DeadlineAt.UTC(),
		RevealedAt:   utcPtr(r.RevealedAt),
		ExpiredAt:    utcPtr(r.ExpiredAt),
		RemindedAt:   utcPtr(r.RemindedAt),
		Participants: parts,
	}
}

func toParticipantModel(dateID uuid.UUID, p dating.Participant) participantModel {
	return participantModel{
		DateID: dateID, UserID: p.UserID, TopicID: p.Topic.ID, Status: string(p.Status),
		JoinedAt: p.JoinedAt, SubmittedAt: p.SubmittedAt, ReceiveDeadlineAt: p.ReceiveDeadlineAt,
		RepresentativePhotoID: p.RepresentativePhotoID, Caption: p.Caption,
	}
}
