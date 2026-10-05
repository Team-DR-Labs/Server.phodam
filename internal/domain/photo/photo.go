// Package photo 는 샷(사진) 레코드와 보관 상태를 정의한다.
package photo

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// MaxSizeBytes 는 업로드 허용 최대 크기다.
	MaxSizeBytes int64 = 20 * 1024 * 1024
	// ContentType 은 업로드에 서명하는 Content-Type 이다.
	ContentType = "image/jpeg"
	// URLTTL 은 presigned URL 유효 시간이다.
	URLTTL = 15 * time.Minute
)

// Status 는 사진 상태다 (정책 §8).
type Status string

const (
	StatusReserved Status = "reserved"
	StatusUploaded Status = "uploaded"
	StatusArchived Status = "archived"
	StatusReceived Status = "received"
	StatusDeleted  Status = "deleted"
)

// Photo 는 셔터 1회로 예약된 사진이다.
type Photo struct {
	ID               uuid.UUID
	DateID           uuid.UUID
	OwnerID          uuid.UUID
	Status           Status
	IsRepresentative bool
	TempKey          string
	PermanentKey     *string
	SizeBytes        *int64
	CreatedAt        time.Time
	UploadedAt       *time.Time
	ReceivedAt       *time.Time
	DeletedAt        *time.Time
	TempPurgedAt     *time.Time
}

// TempKey 는 temp 버킷 객체 키다.
func TempKey(dateID, userID, photoID uuid.UUID) string {
	return fmt.Sprintf("temp/%s/%s/%s.jpg", dateID, userID, photoID)
}

// PermanentKey 는 permanent 버킷 객체 키다.
func PermanentKey(dateID, userID, photoID uuid.UUID) string {
	return fmt.Sprintf("permanent/%s/%s/%s.jpg", dateID, userID, photoID)
}

// NewReserved 는 샷 예약 레코드를 만든다.
func NewReserved(id, dateID, ownerID uuid.UUID, now time.Time) Photo {
	return Photo{
		ID:        id,
		DateID:    dateID,
		OwnerID:   ownerID,
		Status:    StatusReserved,
		TempKey:   TempKey(dateID, ownerID, id),
		CreatedAt: now,
	}
}

// IsOwnedBy 는 소유자 확인이다.
func (p Photo) IsOwnedBy(userID uuid.UUID) bool { return p.OwnerID == userID }

// MarkUploaded 는 temp 업로드 확인 후 상태다.
func (p Photo) MarkUploaded(size int64, now time.Time) Photo {
	next := p
	next.Status = StatusUploaded
	next.SizeBytes = &size
	next.UploadedAt = &now
	return next
}

// Archive 는 대표 사진으로 permanent 에 보관된 상태다.
func (p Photo) Archive() Photo {
	key := PermanentKey(p.DateID, p.OwnerID, p.ID)
	next := p
	next.Status = StatusArchived
	next.IsRepresentative = true
	next.PermanentKey = &key
	return next
}

// MarkReceived 는 기기 저장 ack 후 상태다. 대표 사진은 received_at 만 기록하고 archived 로 둔다.
func (p Photo) MarkReceived(now time.Time) Photo {
	next := p
	if next.ReceivedAt == nil {
		next.ReceivedAt = &now
	}
	if !p.IsRepresentative {
		next.Status = StatusReceived
	}
	return next
}

// IsReceiveAcked 는 이미 수령 ack 를 받았는지 반환한다.
func (p Photo) IsReceiveAcked() bool {
	if p.IsRepresentative {
		return p.ReceivedAt != nil
	}
	return p.Status == StatusReceived
}
