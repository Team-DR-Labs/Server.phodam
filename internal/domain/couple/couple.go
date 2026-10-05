// Package couple 은 커플과 초대 코드 도메인 모델을 정의한다.
package couple

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// InviteCodeLength 는 초대 코드 길이다.
	InviteCodeLength = 8
	// InviteCodeAlphabet 은 헷갈리는 문자(0,O,1,I,L)를 뺀 초대 코드 문자 집합이다.
	InviteCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	// InviteTTL 은 초대 코드 유효 시간이다.
	InviteTTL = 24 * time.Hour
	// StatusActive 는 MVP 에서 쓰는 유일한 커플 상태다. 연결 해제는 미구현이다.
	StatusActive = "active"
)

// Couple 은 두 사용자의 연결이다.
type Couple struct {
	ID          uuid.UUID
	UserAID     uuid.UUID
	UserBID     uuid.UUID
	Status      string
	ConnectedAt time.Time
}

// PartnerOf 는 userID 의 상대 ID 를 반환한다.
func (c Couple) PartnerOf(userID uuid.UUID) uuid.UUID {
	if c.UserAID == userID {
		return c.UserBID
	}
	return c.UserAID
}

// Invite 는 1회용 초대 코드다.
type Invite struct {
	ID        uuid.UUID
	Code      string
	CreatorID uuid.UUID
	ExpiresAt time.Time
	UsedAt    *time.Time
	UsedBy    *uuid.UUID
	RevokedAt *time.Time
	CreatedAt time.Time
}

// UsableAt 은 now 시점에 코드를 쓸 수 있는지 반환한다.
func (i Invite) UsableAt(now time.Time) bool {
	return i.UsedAt == nil && i.RevokedAt == nil && now.Before(i.ExpiresAt)
}

// NormalizeCode 는 사용자가 입력한 코드의 공백을 지우고 대문자로 바꾼다.
// 형식이 맞지 않으면 false 를 반환한다.
func NormalizeCode(raw string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if len(code) != InviteCodeLength {
		return "", false
	}
	for _, r := range code {
		if !strings.ContainsRune(InviteCodeAlphabet, r) {
			return "", false
		}
	}
	return code, true
}
