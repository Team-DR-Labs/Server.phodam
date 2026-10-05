package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
)

type coupleModel struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserAID     uuid.UUID `gorm:"type:uuid;column:user_a_id"`
	UserBID     uuid.UUID `gorm:"type:uuid;column:user_b_id"`
	Status      string
	ConnectedAt time.Time
}

func (coupleModel) TableName() string { return "couples" }

func (m coupleModel) toDomain() couple.Couple {
	return couple.Couple{ID: m.ID, UserAID: m.UserAID, UserBID: m.UserBID, Status: m.Status, ConnectedAt: m.ConnectedAt.UTC()}
}

type inviteModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Code      string
	CreatorID uuid.UUID `gorm:"type:uuid"`
	ExpiresAt time.Time
	UsedAt    *time.Time
	UsedBy    *uuid.UUID `gorm:"type:uuid"`
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (inviteModel) TableName() string { return "couple_invites" }

func (m inviteModel) toDomain() couple.Invite {
	return couple.Invite{
		ID: m.ID, Code: m.Code, CreatorID: m.CreatorID, ExpiresAt: m.ExpiresAt.UTC(),
		UsedAt: utcPtr(m.UsedAt), UsedBy: m.UsedBy, RevokedAt: utcPtr(m.RevokedAt), CreatedAt: m.CreatedAt.UTC(),
	}
}

// CoupleRepository 는 out.CoupleRepository 와 out.InviteRepository 구현체다.
type CoupleRepository struct{ db *gorm.DB }

var (
	_ out.CoupleRepository = (*CoupleRepository)(nil)
	_ out.InviteRepository = (*InviteRepository)(nil)
)

// NewCoupleRepository 는 CoupleRepository 를 생성한다.
func NewCoupleRepository(db *gorm.DB) *CoupleRepository { return &CoupleRepository{db: db} }

// FindActiveByUser 는 사용자의 활성 커플을 찾는다.
func (r *CoupleRepository) FindActiveByUser(ctx context.Context, userID uuid.UUID) (couple.Couple, error) {
	var m coupleModel
	err := conn(ctx, r.db).Where("status = ? AND (user_a_id = ? OR user_b_id = ?)", couple.StatusActive, userID, userID).First(&m).Error
	if err != nil {
		return couple.Couple{}, translate(err)
	}
	return m.toDomain(), nil
}

// Create 는 커플을 저장한다.
func (r *CoupleRepository) Create(ctx context.Context, c couple.Couple) error {
	m := coupleModel{ID: c.ID, UserAID: c.UserAID, UserBID: c.UserBID, Status: c.Status, ConnectedAt: c.ConnectedAt}
	return translate(conn(ctx, r.db).Create(&m).Error)
}

// InviteRepository 는 초대 코드 저장소다.
type InviteRepository struct{ db *gorm.DB }

// NewInviteRepository 는 InviteRepository 를 생성한다.
func NewInviteRepository(db *gorm.DB) *InviteRepository { return &InviteRepository{db: db} }

// RevokeUnused 는 creator 의 미사용 코드를 폐기한다.
func (r *InviteRepository) RevokeUnused(ctx context.Context, creatorID uuid.UUID, now time.Time) error {
	return conn(ctx, r.db).Model(&inviteModel{}).
		Where("creator_id = ? AND used_at IS NULL AND revoked_at IS NULL", creatorID).
		Update("revoked_at", now).Error
}

// Create 는 코드를 저장한다.
func (r *InviteRepository) Create(ctx context.Context, inv couple.Invite) error {
	m := inviteModel{ID: inv.ID, Code: inv.Code, CreatorID: inv.CreatorID, ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt}
	return translate(conn(ctx, r.db).Create(&m).Error)
}

// FindByCodeForUpdate 는 코드를 잠그고 읽는다.
func (r *InviteRepository) FindByCodeForUpdate(ctx context.Context, code string) (couple.Invite, error) {
	var m inviteModel
	err := conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&m).Error
	if err != nil {
		return couple.Invite{}, translate(err)
	}
	return m.toDomain(), nil
}

// MarkUsed 는 코드를 사용 처리한다.
func (r *InviteRepository) MarkUsed(ctx context.Context, id, usedBy uuid.UUID, now time.Time) error {
	return requireRow(conn(ctx, r.db).Model(&inviteModel{}).Where("id = ?", id).
		Updates(map[string]any{"used_at": now, "used_by": usedBy}))
}
