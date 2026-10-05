package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
)

type photoModel struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	DateID           uuid.UUID `gorm:"type:uuid"`
	OwnerID          uuid.UUID `gorm:"type:uuid"`
	Status           string
	IsRepresentative bool
	TempKey          string
	PermanentKey     *string
	SizeBytes        *int64
	UploadedAt       *time.Time
	ReceivedAt       *time.Time
	DeletedAt        *time.Time
	TempPurgedAt     *time.Time
	CreatedAt        time.Time
}

func (photoModel) TableName() string { return "photos" }

func (m photoModel) toDomain() photo.Photo {
	return photo.Photo{
		ID: m.ID, DateID: m.DateID, OwnerID: m.OwnerID, Status: photo.Status(m.Status),
		IsRepresentative: m.IsRepresentative, TempKey: m.TempKey, PermanentKey: m.PermanentKey,
		SizeBytes: m.SizeBytes, CreatedAt: m.CreatedAt.UTC(), UploadedAt: utcPtr(m.UploadedAt),
		ReceivedAt: utcPtr(m.ReceivedAt), DeletedAt: utcPtr(m.DeletedAt), TempPurgedAt: utcPtr(m.TempPurgedAt),
	}
}

func toPhotoModel(p photo.Photo) photoModel {
	return photoModel{
		ID: p.ID, DateID: p.DateID, OwnerID: p.OwnerID, Status: string(p.Status),
		IsRepresentative: p.IsRepresentative, TempKey: p.TempKey, PermanentKey: p.PermanentKey,
		SizeBytes: p.SizeBytes, UploadedAt: p.UploadedAt, ReceivedAt: p.ReceivedAt,
		DeletedAt: p.DeletedAt, TempPurgedAt: p.TempPurgedAt, CreatedAt: p.CreatedAt,
	}
}

func toPhotos(rows []photoModel) []photo.Photo {
	res := make([]photo.Photo, 0, len(rows))
	for _, m := range rows {
		res = append(res, m.toDomain())
	}
	return res
}

// PhotoRepository 는 out.PhotoRepository 구현체다.
type PhotoRepository struct{ db *gorm.DB }

var _ out.PhotoRepository = (*PhotoRepository)(nil)

// NewPhotoRepository 는 PhotoRepository 를 생성한다.
func NewPhotoRepository(db *gorm.DB) *PhotoRepository { return &PhotoRepository{db: db} }

// Create 는 사진 레코드를 저장한다.
func (r *PhotoRepository) Create(ctx context.Context, p photo.Photo) error {
	m := toPhotoModel(p)
	return translate(conn(ctx, r.db).Create(&m).Error)
}

// Get 은 사진을 읽는다.
func (r *PhotoRepository) Get(ctx context.Context, id uuid.UUID) (photo.Photo, error) {
	var m photoModel
	if err := conn(ctx, r.db).First(&m, "id = ?", id).Error; err != nil {
		return photo.Photo{}, translate(err)
	}
	return m.toDomain(), nil
}

// GetForUpdate 는 사진 행을 잠그고 읽는다.
func (r *PhotoRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (photo.Photo, error) {
	var m photoModel
	err := conn(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).First(&m, "id = ?", id).Error
	if err != nil {
		return photo.Photo{}, translate(err)
	}
	return m.toDomain(), nil
}

// Update 는 상태 관련 필드를 저장한다.
func (r *PhotoRepository) Update(ctx context.Context, p photo.Photo) error {
	m := toPhotoModel(p)
	return requireRow(conn(ctx, r.db).Model(&photoModel{}).Where("id = ?", p.ID).Updates(map[string]any{
		"status": m.Status, "is_representative": m.IsRepresentative, "permanent_key": m.PermanentKey,
		"size_bytes": m.SizeBytes, "uploaded_at": m.UploadedAt, "received_at": m.ReceivedAt,
		"deleted_at": m.DeletedAt, "temp_purged_at": m.TempPurgedAt,
	}))
}

// ListByOwner 는 이 데이트의 owner 사진 중 statuses 인 것을 촬영 순으로 반환한다.
func (r *PhotoRepository) ListByOwner(ctx context.Context, dateID, ownerID uuid.UUID, statuses ...photo.Status) ([]photo.Photo, error) {
	var rows []photoModel
	err := conn(ctx, r.db).Where("date_id = ? AND owner_id = ? AND status IN ?", dateID, ownerID, statusStrings(statuses)).
		Order("created_at, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return toPhotos(rows), nil
}

// CountByOwner 는 이 데이트에서 owner 가 예약한 샷 수다 (삭제 포함).
func (r *PhotoRepository) CountByOwner(ctx context.Context, dateID, ownerID uuid.UUID) (int, error) {
	var n int64
	err := conn(ctx, r.db).Model(&photoModel{}).Where("date_id = ? AND owner_id = ?", dateID, ownerID).Count(&n).Error
	return int(n), err
}

// DeleteReserved 는 owner 의 reserved 사진을 deleted 로 바꾸고 바뀐 사진을 반환한다.
func (r *PhotoRepository) DeleteReserved(ctx context.Context, dateID, ownerID uuid.UUID, now time.Time) ([]photo.Photo, error) {
	var rows []photoModel
	err := conn(ctx, r.db).Raw(`UPDATE photos SET status = 'deleted', deleted_at = ?
		WHERE date_id = ? AND owner_id = ? AND status = 'reserved' RETURNING *`, now, dateID, ownerID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return toPhotos(rows), nil
}

// MarkTempPurged 는 temp 사본 삭제를 기록한다.
func (r *PhotoRepository) MarkTempPurged(ctx context.Context, id uuid.UUID, now time.Time) error {
	return conn(ctx, r.db).Model(&photoModel{}).Where("id = ?", id).Update("temp_purged_at", now).Error
}

// MarkDeleted 는 from 상태일 때만 deleted 로 바꾼다.
func (r *PhotoRepository) MarkDeleted(ctx context.Context, id uuid.UUID, now time.Time, from ...photo.Status) (bool, error) {
	res := conn(ctx, r.db).Model(&photoModel{}).Where("id = ? AND status IN ?", id, statusStrings(from)).
		Updates(map[string]any{"status": string(photo.StatusDeleted), "deleted_at": now})
	return res.RowsAffected > 0, res.Error
}

// ListExpiredLeftovers 는 만료 데이트에서 미제출 참여자의 reserved/uploaded 사진이다.
func (r *PhotoRepository) ListExpiredLeftovers(ctx context.Context, limit int) ([]photo.Photo, error) {
	return r.query(ctx, `SELECT ph.* FROM photos ph
		JOIN dates d ON d.id = ph.date_id
		JOIN date_participants p ON p.date_id = ph.date_id AND p.user_id = ph.owner_id
		WHERE d.status = 'expired' AND p.status <> 'submitted' AND ph.status IN ('reserved', 'uploaded')
		ORDER BY ph.created_at LIMIT ?`, limit)
}

// ListReceiveOverdue 는 수령 기한이 지난 uploaded 비대표 사진이다.
func (r *PhotoRepository) ListReceiveOverdue(ctx context.Context, now time.Time, limit int) ([]photo.Photo, error) {
	return r.query(ctx, `SELECT ph.* FROM photos ph
		JOIN date_participants p ON p.date_id = ph.date_id AND p.user_id = ph.owner_id
		WHERE p.status = 'submitted' AND p.receive_deadline_at <= ?
		  AND ph.status = 'uploaded' AND ph.is_representative = false
		ORDER BY ph.created_at LIMIT ?`, now, limit)
}

// ListArchivedWithTemp 는 temp 사본이 남은 대표 사진이다.
func (r *PhotoRepository) ListArchivedWithTemp(ctx context.Context, limit int) ([]photo.Photo, error) {
	return r.query(ctx, `SELECT * FROM photos WHERE status = 'archived' AND temp_purged_at IS NULL
		ORDER BY created_at LIMIT ?`, limit)
}

func (r *PhotoRepository) query(ctx context.Context, query string, args ...any) ([]photo.Photo, error) {
	var rows []photoModel
	if err := conn(ctx, r.db).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return toPhotos(rows), nil
}

func statusStrings(ss []photo.Status) []string {
	res := make([]string, 0, len(ss))
	for _, s := range ss {
		res = append(res, string(s))
	}
	return res
}
