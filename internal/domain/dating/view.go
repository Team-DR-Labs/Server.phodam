package dating

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// CaptionMaxRunes 는 제출 글 최대 길이(유니코드 문자 수)다.
const CaptionMaxRunes = 200

// NormalizeCaption 은 앞뒤 공백을 지우고 길이를 검사한다. 빈 문자열은 nil 이다.
func NormalizeCaption(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	c := strings.TrimSpace(*raw)
	if c == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(c) > CaptionMaxRunes {
		return nil, apperr.New(apperr.ValidationFailed, "caption must be at most 200 characters")
	}
	return &c, nil
}

// MyParticipation 은 요청자 본인의 참여 정보다. Topic 은 joined 이후에만 채운다.
type MyParticipation struct {
	Status            ParticipantStatus
	Topic             *Topic
	JoinedAt          *time.Time
	SubmittedAt       *time.Time
	ReceiveDeadlineAt *time.Time
	ShotCount         int
}

// PartnerParticipation 은 상대 정보다. Topic 은 revealed 일 때만 채운다.
type PartnerParticipation struct {
	User   user.Profile
	Status ParticipantStatus
	Topic  *Topic
}

// View 는 요청자 시점의 데이트 응답 모델이다. 공개 규칙(정책 §5)을 여기서 적용한다.
type View struct {
	ID          uuid.UUID
	Status      Status
	Theme       Theme
	StartedByMe bool
	StartedAt   time.Time
	DeadlineAt  time.Time
	RevealedAt  *time.Time
	Me          MyParticipation
	Partner     PartnerParticipation
}

// ViewFor 는 viewer 시점의 View 를 만든다. 상대 주제는 revealed 전까지 넣지 않는다.
func (d Date) ViewFor(viewer uuid.UUID, partner user.Profile, shotCount int, now time.Time) View {
	me, _ := d.Participant(viewer)
	other, _ := d.Partner(viewer)
	status := d.EffectiveStatus(now)

	var myTopic *Topic
	if me.Status != ParticipantAssigned {
		t := me.Topic
		myTopic = &t
	}
	var partnerTopic *Topic
	if status == StatusRevealed {
		t := other.Topic
		partnerTopic = &t
	}
	return View{
		ID:          d.ID,
		Status:      status,
		Theme:       d.Theme,
		StartedByMe: d.StartedBy == viewer,
		StartedAt:   d.StartedAt,
		DeadlineAt:  d.DeadlineAt,
		RevealedAt:  d.RevealedAt,
		Me: MyParticipation{
			Status:            me.Status,
			Topic:             myTopic,
			JoinedAt:          me.JoinedAt,
			SubmittedAt:       me.SubmittedAt,
			ReceiveDeadlineAt: me.ReceiveDeadlineAt,
			ShotCount:         shotCount,
		},
		Partner: PartnerParticipation{User: partner, Status: other.Status, Topic: partnerTopic},
	}
}
