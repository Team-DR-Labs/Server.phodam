package http

import (
	"time"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// Timestamp 는 RFC 3339 UTC(`Z`) 로 직렬화되는 시각이다.
type Timestamp time.Time

// MarshalJSON 은 초 단위 RFC 3339 UTC 문자열로 쓴다.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).UTC().Format(time.RFC3339) + `"`), nil
}

func ts(t time.Time) Timestamp { return Timestamp(t) }

func tsPtr(t *time.Time) *Timestamp {
	if t == nil {
		return nil
	}
	v := Timestamp(*t)
	return &v
}

// UserResponse 는 User 스키마다.
type UserResponse struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

func toUser(p user.Profile) UserResponse {
	return UserResponse{ID: p.ID.String(), Nickname: p.Nickname}
}

// CoupleResponse 는 Couple 스키마다.
type CoupleResponse struct {
	ID          string       `json:"id"`
	Partner     UserResponse `json:"partner"`
	ConnectedAt Timestamp    `json:"connected_at"`
}

func toCouple(c in.CoupleView) CoupleResponse {
	return CoupleResponse{ID: c.ID.String(), Partner: toUser(c.Partner), ConnectedAt: ts(c.ConnectedAt)}
}
