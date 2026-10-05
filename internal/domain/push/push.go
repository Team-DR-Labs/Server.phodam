// Package push 는 푸시 알림 메시지 모델을 정의한다.
package push

import "github.com/google/uuid"

// Type 은 앱이 분기하는 푸시 종류다 (data payload 의 type).
type Type string

const (
	TypeDateStarted      Type = "date_started"
	TypePartnerSubmitted Type = "partner_submitted"
	TypeDateRevealed     Type = "date_revealed"
	TypeDeadlineSoon     Type = "deadline_soon"
)

// Message 는 발송할 알림이다. Data 는 {"type", "date_id"} 다.
type Message struct {
	Type   Type
	DateID uuid.UUID
	Title  string
	Body   string
}

// Data 는 FCM data payload 를 반환한다.
func (m Message) Data() map[string]string {
	return map[string]string{"type": string(m.Type), "date_id": m.DateID.String()}
}

// NewMessage 는 서버가 정한 문구로 알림을 만든다.
func NewMessage(t Type, dateID uuid.UUID) Message {
	title, body := copyFor(t)
	return Message{Type: t, DateID: dateID, Title: title, Body: body}
}

func copyFor(t Type) (string, string) {
	switch t {
	case TypeDateStarted:
		return "새 데이트가 시작됐어요", "오늘의 주제를 확인하고 함께 담아 보세요."
	case TypePartnerSubmitted:
		return "상대가 일기를 제출했어요", "내 사진도 골라 제출하면 함께 열어 볼 수 있어요."
	case TypeDateRevealed:
		return "일기가 공개됐어요", "서로의 대표 사진과 글을 확인해 보세요."
	case TypeDeadlineSoon:
		return "마감이 6시간 남았어요", "대표 사진을 골라 제출해 주세요."
	default:
		return "포담", ""
	}
}
