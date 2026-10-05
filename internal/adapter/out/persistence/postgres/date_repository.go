package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
)

const dateSelect = `SELECT d.id, d.couple_id, d.theme_id, d.status, d.started_by, d.started_at, d.deadline_at,
	d.revealed_at, d.expired_at, d.reminded_at, t.title AS theme_title
	FROM dates d JOIN themes t ON t.id = d.theme_id `

// DateRepository 는 out.DateRepository 와 out.ThemeRepository 구현체다.
type DateRepository struct{ db *gorm.DB }

var (
	_ out.DateRepository  = (*DateRepository)(nil)
	_ out.ThemeRepository = (*DateRepository)(nil)
)

// NewDateRepository 는 DateRepository 를 생성한다.
func NewDateRepository(db *gorm.DB) *DateRepository { return &DateRepository{db: db} }

// ListThemes 는 시드된 테마를 반환한다.
func (r *DateRepository) ListThemes(ctx context.Context) ([]dating.Theme, error) {
	var rows []themeModel
	if err := conn(ctx, r.db).Order("sort_order").Find(&rows).Error; err != nil {
		return nil, err
	}
	res := make([]dating.Theme, 0, len(rows))
	for _, t := range rows {
		res = append(res, dating.Theme{ID: t.ID, Title: t.Title})
	}
	return res, nil
}

// ListTopics 는 테마의 주제를 반환한다.
func (r *DateRepository) ListTopics(ctx context.Context, themeID uuid.UUID) ([]dating.Topic, error) {
	var rows []topicModel
	if err := conn(ctx, r.db).Where("theme_id = ?", themeID).Order("sort_order").Find(&rows).Error; err != nil {
		return nil, err
	}
	res := make([]dating.Topic, 0, len(rows))
	for _, t := range rows {
		res = append(res, dating.Topic{ID: t.ID, Title: t.Title})
	}
	return res, nil
}

// Create 는 데이트와 참여자를 저장한다.
func (r *DateRepository) Create(ctx context.Context, d dating.Date) error {
	db := conn(ctx, r.db)
	m := dateModel{
		ID: d.ID, CoupleID: d.CoupleID, ThemeID: d.Theme.ID, Status: string(d.Status),
		StartedBy: d.StartedBy, StartedAt: d.StartedAt, DeadlineAt: d.DeadlineAt,
	}
	if err := db.Create(&m).Error; err != nil {
		return translate(err)
	}
	parts := make([]participantModel, 0, len(d.Participants))
	for _, p := range d.Participants {
		parts = append(parts, toParticipantModel(d.ID, p))
	}
	return translate(db.Create(&parts).Error)
}

// Get 은 데이트를 읽는다.
func (r *DateRepository) Get(ctx context.Context, id uuid.UUID) (dating.Date, error) {
	return r.getOne(ctx, dateSelect+`WHERE d.id = ?`, id)
}

// GetForUpdate 는 데이트 행을 잠그고 읽는다.
func (r *DateRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (dating.Date, error) {
	return r.getOne(ctx, dateSelect+`WHERE d.id = ? FOR UPDATE OF d`, id)
}

// FindInProgressByCouple 은 커플의 in_progress 데이트를 찾는다.
func (r *DateRepository) FindInProgressByCouple(ctx context.Context, coupleID uuid.UUID) (dating.Date, error) {
	return r.getOne(ctx, dateSelect+`WHERE d.couple_id = ? AND d.status = 'in_progress'`, coupleID)
}

func (r *DateRepository) getOne(ctx context.Context, query string, args ...any) (dating.Date, error) {
	dates, err := r.load(ctx, query, args...)
	if err != nil {
		return dating.Date{}, err
	}
	if len(dates) == 0 {
		return dating.Date{}, out.ErrNotFound
	}
	return dates[0], nil
}

// load 는 dateSelect 기반 조회 결과에 참여자를 붙여 반환한다 (조회 순서 유지).
func (r *DateRepository) load(ctx context.Context, query string, args ...any) ([]dating.Date, error) {
	db := conn(ctx, r.db)
	var rows []dateRow
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	dateIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		dateIDs = append(dateIDs, row.ID)
	}
	var parts []participantRow
	err := db.Raw(`SELECT p.*, tp.title AS topic_title FROM date_participants p
		JOIN topics tp ON tp.id = p.topic_id WHERE p.date_id IN ? ORDER BY p.created_at, p.user_id`, dateIDs).Scan(&parts).Error
	if err != nil {
		return nil, err
	}
	byDate := make(map[uuid.UUID][]dating.Participant, len(rows))
	for _, p := range parts {
		byDate[p.DateID] = append(byDate[p.DateID], p.toDomain())
	}
	res := make([]dating.Date, 0, len(rows))
	for _, row := range rows {
		res = append(res, row.toDomain(byDate[row.ID]))
	}
	return res, nil
}

// ThemeHistory 는 커플이 쓴 테마와 직전 테마를 반환한다.
func (r *DateRepository) ThemeHistory(ctx context.Context, coupleID uuid.UUID) (out.ThemeHistory, error) {
	var rows []dateModel
	err := conn(ctx, r.db).Select("theme_id", "started_at").Where("couple_id = ?", coupleID).
		Order("started_at DESC").Find(&rows).Error
	if err != nil {
		return out.ThemeHistory{}, err
	}
	h := out.ThemeHistory{Used: make(map[uuid.UUID]bool, len(rows))}
	for i, row := range rows {
		if i == 0 {
			last := row.ThemeID
			h.Last = &last
		}
		h.Used[row.ThemeID] = true
	}
	return h, nil
}

// UpdateParticipant 는 참여자 상태 필드를 저장한다.
func (r *DateRepository) UpdateParticipant(ctx context.Context, dateID uuid.UUID, p dating.Participant) error {
	m := toParticipantModel(dateID, p)
	return requireRow(conn(ctx, r.db).Model(&participantModel{}).
		Where("date_id = ? AND user_id = ?", dateID, p.UserID).
		Updates(map[string]any{
			"status": m.Status, "joined_at": m.JoinedAt, "submitted_at": m.SubmittedAt,
			"receive_deadline_at": m.ReceiveDeadlineAt, "representative_photo_id": m.RepresentativePhotoID,
			"caption": m.Caption,
		}))
}

// MarkRevealed 는 in_progress 데이트를 revealed 로 바꾼다.
func (r *DateRepository) MarkRevealed(ctx context.Context, id uuid.UUID, at time.Time) error {
	return requireRow(conn(ctx, r.db).Model(&dateModel{}).
		Where("id = ? AND status = 'in_progress'", id).
		Updates(map[string]any{"status": string(dating.StatusRevealed), "revealed_at": at}))
}

// Expire 는 마감이 지난 in_progress 데이트 하나를 expired 로 바꾼다.
func (r *DateRepository) Expire(ctx context.Context, id uuid.UUID, now time.Time) error {
	return conn(ctx, r.db).Model(&dateModel{}).
		Where("id = ? AND status = 'in_progress' AND deadline_at <= ?", id, now).
		Updates(map[string]any{"status": string(dating.StatusExpired), "expired_at": now}).Error
}

// ListSubmittedByUser 는 userID 가 제출한 데이트를 (started_at, id) 내림차순으로 읽는다.
func (r *DateRepository) ListSubmittedByUser(ctx context.Context, userID uuid.UUID, after *diary.Cursor, limit int) ([]dating.Date, error) {
	query := dateSelect + `JOIN date_participants me ON me.date_id = d.id
		WHERE me.user_id = ? AND me.status = 'submitted'`
	args := []any{userID}
	if after != nil {
		query += ` AND (d.started_at, d.id) < (?, ?)`
		args = append(args, after.StartedAt, after.DateID)
	}
	query += ` ORDER BY d.started_at DESC, d.id DESC LIMIT ?`
	args = append(args, limit)
	return r.load(ctx, query, args...)
}

// ExpireDue 는 마감이 지난 in_progress 데이트를 SKIP LOCKED 로 잡아 expired 로 바꾼다.
func (r *DateRepository) ExpireDue(ctx context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	var rows []idRow
	err := conn(ctx, r.db).Raw(`UPDATE dates SET status = 'expired', expired_at = ?
		WHERE id IN (
			SELECT id FROM dates WHERE status = 'in_progress' AND deadline_at <= ?
			ORDER BY deadline_at LIMIT ? FOR UPDATE SKIP LOCKED
		) RETURNING id`, now, now, limit).Scan(&rows).Error
	return ids(rows), err
}

// ClaimReminders 는 마감 6시간 전에 들어선 데이트의 reminded_at 을 기록하고 미제출 참여자를 반환한다.
func (r *DateRepository) ClaimReminders(ctx context.Context, now time.Time, limit int) ([]out.Reminder, error) {
	db := conn(ctx, r.db)
	var rows []idRow
	err := db.Raw(`UPDATE dates SET reminded_at = ?
		WHERE id IN (
			SELECT id FROM dates
			WHERE status = 'in_progress' AND reminded_at IS NULL
			  AND deadline_at <= ? AND deadline_at > ?
			ORDER BY deadline_at LIMIT ? FOR UPDATE SKIP LOCKED
		) RETURNING id`, now, now.Add(dating.ReminderLead), now, limit).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	dateIDs := ids(rows)
	var parts []participantModel
	err = db.Where("date_id IN ? AND status <> ?", dateIDs, string(dating.ParticipantSubmitted)).Find(&parts).Error
	if err != nil {
		return nil, err
	}
	byDate := make(map[uuid.UUID][]uuid.UUID, len(dateIDs))
	for _, p := range parts {
		byDate[p.DateID] = append(byDate[p.DateID], p.UserID)
	}
	res := make([]out.Reminder, 0, len(dateIDs))
	for _, id := range dateIDs {
		res = append(res, out.Reminder{DateID: id, UserIDs: byDate[id]})
	}
	return res, nil
}
