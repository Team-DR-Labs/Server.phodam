package postgres

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// UserRepository 는 사용자·식별자·원장·리프레시 토큰·기기 포트를 구현한다.
type UserRepository struct {
	db *gorm.DB
}

var (
	_ out.UserRepository         = (*UserRepository)(nil)
	_ out.IdentityRepository     = (*IdentityRepository)(nil)
	_ out.FilmLedgerRepository   = (*LedgerRepository)(nil)
	_ out.RefreshTokenRepository = (*RefreshTokenRepository)(nil)
	_ out.DeviceRepository       = (*DeviceRepository)(nil)
)

// NewUserRepository 는 UserRepository 를 생성한다.
func NewUserRepository(db *gorm.DB) *UserRepository { return &UserRepository{db: db} }

// Create 는 사용자를 저장한다.
func (r *UserRepository) Create(ctx context.Context, u user.User) error {
	m := userModel{ID: u.ID, Nickname: u.Nickname, FilmBalance: u.FilmBalance, Status: u.Status, CreatedAt: u.CreatedAt}
	return translate(conn(ctx, r.db).Create(&m).Error)
}

// Get 은 사용자를 읽는다.
func (r *UserRepository) Get(ctx context.Context, id uuid.UUID) (user.User, error) {
	var m userModel
	if err := conn(ctx, r.db).First(&m, "id = ?", id).Error; err != nil {
		return user.User{}, translate(err)
	}
	return m.toDomain(), nil
}

// LockForUpdate 는 교착을 피하려고 id 순서로 사용자 행을 잠근다.
func (r *UserRepository) LockForUpdate(ctx context.Context, ids ...uuid.UUID) error {
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b uuid.UUID) int { return compareUUID(a, b) })
	var rows []userModel
	return conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", sorted).Order("id").Find(&rows).Error
}

// UpdateNickname 은 닉네임을 바꾼다.
func (r *UserRepository) UpdateNickname(ctx context.Context, id uuid.UUID, nickname string) error {
	return requireRow(conn(ctx, r.db).Model(&userModel{}).Where("id = ?", id).Update("nickname", nickname))
}

// DecrementFilm 은 잔액이 남아 있을 때만 1 차감한다.
func (r *UserRepository) DecrementFilm(ctx context.Context, id uuid.UUID) (int, bool, error) {
	var balances []int
	err := conn(ctx, r.db).Raw(
		`UPDATE users SET film_balance = film_balance - 1 WHERE id = ? AND film_balance > 0 RETURNING film_balance`, id,
	).Scan(&balances).Error
	if err != nil {
		return 0, false, err
	}
	if len(balances) == 0 {
		return 0, false, nil
	}
	return balances[0], true, nil
}

// AddFilm 은 잔액에 n 을 더한다.
func (r *UserRepository) AddFilm(ctx context.Context, id uuid.UUID, n int) (int, error) {
	var balances []int
	err := conn(ctx, r.db).Raw(
		`UPDATE users SET film_balance = film_balance + ? WHERE id = ? RETURNING film_balance`, n, id,
	).Scan(&balances).Error
	if err != nil {
		return 0, err
	}
	if len(balances) == 0 {
		return 0, out.ErrNotFound
	}
	return balances[0], nil
}

func compareUUID(a, b uuid.UUID) int {
	for i := range a {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return 0
}

// IdentityRepository 는 외부 로그인 식별자 저장소다.
type IdentityRepository struct{ db *gorm.DB }

// NewIdentityRepository 는 IdentityRepository 를 생성한다.
func NewIdentityRepository(db *gorm.DB) *IdentityRepository { return &IdentityRepository{db: db} }

// FindUserID 는 (provider, subject) 의 사용자 ID 를 찾는다.
func (r *IdentityRepository) FindUserID(ctx context.Context, p user.Provider, subject string) (uuid.UUID, error) {
	var m identityModel
	err := conn(ctx, r.db).Where("provider = ? AND subject = ?", string(p), subject).First(&m).Error
	if err != nil {
		return uuid.Nil, translate(err)
	}
	return m.UserID, nil
}

// Create 는 식별자를 저장한다.
func (r *IdentityRepository) Create(ctx context.Context, i user.Identity) error {
	m := identityModel{ID: uuid.New(), UserID: i.UserID, Provider: string(i.Provider), Subject: i.Subject}
	return translate(conn(ctx, r.db).Create(&m).Error)
}

// LedgerRepository 는 필름 원장 저장소다.
type LedgerRepository struct{ db *gorm.DB }

// NewLedgerRepository 는 LedgerRepository 를 생성한다.
func NewLedgerRepository(db *gorm.DB) *LedgerRepository { return &LedgerRepository{db: db} }

// Append 는 원장 기록을 추가한다.
func (r *LedgerRepository) Append(ctx context.Context, e user.LedgerEntry) error {
	m := ledgerModel{ID: e.ID, UserID: e.UserID, Delta: e.Delta, Reason: string(e.Reason), RefID: e.RefID, Memo: e.Memo, CreatedAt: e.CreatedAt}
	return translate(conn(ctx, r.db).Create(&m).Error)
}

// RefreshTokenRepository 는 리프레시 토큰 저장소다.
type RefreshTokenRepository struct{ db *gorm.DB }

// NewRefreshTokenRepository 는 RefreshTokenRepository 를 생성한다.
func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

// Create 는 토큰 해시를 저장한다.
func (r *RefreshTokenRepository) Create(ctx context.Context, t out.RefreshToken) error {
	m := refreshTokenModel{ID: t.ID, UserID: t.UserID, TokenHash: t.TokenHash, ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt}
	return translate(conn(ctx, r.db).Create(&m).Error)
}

// Consume 은 유효한 토큰을 원자적으로 폐기하고 소유자를 반환한다.
func (r *RefreshTokenRepository) Consume(ctx context.Context, hash string, now time.Time) (uuid.UUID, error) {
	var ids []uuid.UUID
	err := conn(ctx, r.db).Raw(
		`UPDATE refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL AND expires_at > ? RETURNING user_id`,
		now, hash, now,
	).Scan(&ids).Error
	if err != nil {
		return uuid.Nil, err
	}
	if len(ids) == 0 {
		return uuid.Nil, out.ErrNotFound
	}
	return ids[0], nil
}

// Revoke 는 userID 소유 토큰을 폐기한다.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, userID uuid.UUID, hash string, now time.Time) error {
	return conn(ctx, r.db).Model(&refreshTokenModel{}).
		Where("token_hash = ? AND user_id = ? AND revoked_at IS NULL", hash, userID).
		Update("revoked_at", now).Error
}

// DeviceRepository 는 FCM 기기 토큰 저장소다.
type DeviceRepository struct{ db *gorm.DB }

// NewDeviceRepository 는 DeviceRepository 를 생성한다.
func NewDeviceRepository(db *gorm.DB) *DeviceRepository { return &DeviceRepository{db: db} }

// Upsert 는 토큰을 등록하거나 소유자·플랫폼을 바꾼다.
func (r *DeviceRepository) Upsert(ctx context.Context, d user.Device) error {
	return conn(ctx, r.db).Exec(
		`INSERT INTO user_devices (user_id, fcm_token, platform) VALUES (?, ?, ?)
		 ON CONFLICT (fcm_token) DO UPDATE SET user_id = EXCLUDED.user_id, platform = EXCLUDED.platform`,
		d.UserID, d.Token, string(d.Platform),
	).Error
}

// ListTokens 는 사용자들의 토큰을 반환한다.
func (r *DeviceRepository) ListTokens(ctx context.Context, userIDs ...uuid.UUID) ([]string, error) {
	var tokens []string
	if len(userIDs) == 0 {
		return tokens, nil
	}
	err := conn(ctx, r.db).Table("user_devices").Where("user_id IN ?", userIDs).Pluck("fcm_token", &tokens).Error
	return tokens, err
}

// DeleteTokens 는 토큰을 지운다.
func (r *DeviceRepository) DeleteTokens(ctx context.Context, tokens ...string) error {
	if len(tokens) == 0 {
		return nil
	}
	return conn(ctx, r.db).Exec(`DELETE FROM user_devices WHERE fcm_token IN ?`, tokens).Error
}
