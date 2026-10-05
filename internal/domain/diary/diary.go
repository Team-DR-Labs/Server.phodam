// Package diary 는 일기 열람 모델과 공개 규칙(정책 §10)을 정의한다.
package diary

import (
	"time"
	// distroless 이미지에는 시스템 tzdata 가 없으므로 내장한다.
	_ "time/tzdata"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

const (
	// DefaultLimit 는 목록 기본 개수다.
	DefaultLimit = 20
	// MaxLimit 는 목록 최대 개수다.
	MaxLimit = 50
)

// Visibility 는 일기 공개 상태다.
type Visibility string

const (
	VisibilityShared  Visibility = "shared"
	VisibilityWaiting Visibility = "waiting"
	VisibilityPrivate Visibility = "private"
)

// seoul 은 local_date 계산용 시간대다.
var seoul = mustLoadSeoul()

func mustLoadSeoul() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		panic("load Asia/Seoul: " + err.Error())
	}
	return loc
}

// LocalDate 는 started_at 의 Asia/Seoul 날짜(YYYY-MM-DD)다.
func LocalDate(t time.Time) string { return t.In(seoul).Format(time.DateOnly) }

// VisibilityFor 는 viewer 가 이 데이트를 일기로 볼 수 있는지와 공개 상태를 반환한다.
// 내가 제출하지 않았으면 볼 수 없다.
func VisibilityFor(d dating.Date, viewer uuid.UUID, now time.Time) (Visibility, bool) {
	me, ok := d.Participant(viewer)
	if !ok || !me.HasSubmitted() {
		return "", false
	}
	switch d.EffectiveStatus(now) {
	case dating.StatusRevealed:
		return VisibilityShared, true
	case dating.StatusExpired:
		return VisibilityPrivate, true
	default:
		return VisibilityWaiting, true
	}
}

// URL 은 presigned GET URL 과 만료 시각이다.
type URL struct {
	URL       string
	ExpiresAt time.Time
}

// ListItem 은 일기 목록 항목이다.
type ListItem struct {
	DateID     uuid.UUID
	LocalDate  string
	Theme      dating.Theme
	Visibility Visibility
	StartedAt  time.Time
	Thumbnail  URL
}

// Page 는 일기 목록 페이지다.
type Page struct {
	Items      []ListItem
	NextCursor *string
}

// Entry 는 한 사람의 일기 내용이다.
type Entry struct {
	Author      user.Profile
	IsMe        bool
	Topic       dating.Topic
	Caption     *string
	Photo       URL
	SubmittedAt time.Time
}

// Detail 은 일기 상세다. shared 면 2개(내 것 먼저), 그 외에는 내 것 1개다.
type Detail struct {
	DateID     uuid.UUID
	LocalDate  string
	Theme      dating.Theme
	Visibility Visibility
	StartedAt  time.Time
	Entries    []Entry
}

// Cursor 는 started_at 내림차순 페이지 위치다.
type Cursor struct {
	StartedAt time.Time
	DateID    uuid.UUID
}
