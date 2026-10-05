package postgres

import (
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

type userModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Nickname    string
	FilmBalance int
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time `gorm:"->"`
}

func (userModel) TableName() string { return "users" }

func (m userModel) toDomain() user.User {
	return user.User{ID: m.ID, Nickname: m.Nickname, FilmBalance: m.FilmBalance, Status: m.Status, CreatedAt: m.CreatedAt.UTC()}
}

type identityModel struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID   uuid.UUID `gorm:"type:uuid"`
	Provider string
	Subject  string
}

func (identityModel) TableName() string { return "user_identities" }

type ledgerModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid"`
	Delta     int
	Reason    string
	RefID     *uuid.UUID `gorm:"type:uuid"`
	Memo      *string
	CreatedAt time.Time
}

func (ledgerModel) TableName() string { return "film_ledger" }

type refreshTokenModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid"`
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (refreshTokenModel) TableName() string { return "refresh_tokens" }

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
