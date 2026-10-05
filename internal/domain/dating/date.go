// Package dating 은 데이트(커플의 촬영 단위)와 참여자 상태 머신을 정의한다.
// 시각 비교는 모두 호출자가 넘긴 now(Clock) 기준이다.
package dating

import (
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
)

const (
	// SubmitWindow 는 시작부터 제출 마감까지의 시간이다.
	SubmitWindow = 72 * time.Hour
	// ReceiveWindow 는 제출부터 수령 마감까지의 시간이다.
	ReceiveWindow = 7 * 24 * time.Hour
	// ReminderLead 는 마감 임박 알림을 보내는 마감 전 시간이다.
	ReminderLead = 6 * time.Hour
)

// Status 는 데이트 상태다.
type Status string

const (
	StatusInProgress Status = "in_progress"
	StatusRevealed   Status = "revealed"
	StatusExpired    Status = "expired"
)

// ParticipantStatus 는 참여자 상태다.
type ParticipantStatus string

const (
	ParticipantAssigned  ParticipantStatus = "assigned"
	ParticipantJoined    ParticipantStatus = "joined"
	ParticipantSubmitted ParticipantStatus = "submitted"
)

// Theme 은 데이트 테마다.
type Theme struct {
	ID    uuid.UUID
	Title string
}

// Topic 은 테마 안의 촬영 주제다.
type Topic struct {
	ID    uuid.UUID
	Title string
}

// Participant 는 데이트별 사용자 상태다.
type Participant struct {
	UserID                uuid.UUID
	Topic                 Topic
	Status                ParticipantStatus
	JoinedAt              *time.Time
	SubmittedAt           *time.Time
	ReceiveDeadlineAt     *time.Time
	RepresentativePhotoID *uuid.UUID
	Caption               *string
}

// HasSubmitted 는 제출 여부다.
func (p Participant) HasSubmitted() bool { return p.Status == ParticipantSubmitted }

// CanReceiveAt 은 now 시점에 수령 가능한지 반환한다 (제출 후 7일 이내).
func (p Participant) CanReceiveAt(now time.Time) bool {
	return p.HasSubmitted() && p.ReceiveDeadlineAt != nil && now.Before(*p.ReceiveDeadlineAt)
}

// Date 는 커플의 촬영 단위 하나다. Participants 는 항상 2명이다.
type Date struct {
	ID           uuid.UUID
	CoupleID     uuid.UUID
	Theme        Theme
	Status       Status
	StartedBy    uuid.UUID
	StartedAt    time.Time
	DeadlineAt   time.Time
	RevealedAt   *time.Time
	ExpiredAt    *time.Time
	RemindedAt   *time.Time
	Participants []Participant
}

// StartParams 는 데이트 시작에 필요한 값이다.
type StartParams struct {
	ID           uuid.UUID
	CoupleID     uuid.UUID
	Theme        Theme
	Starter      uuid.UUID
	Partner      uuid.UUID
	StarterTopic Topic
	PartnerTopic Topic
	Now          time.Time
}

// Start 는 새 데이트를 만든다. 시작자는 즉시 joined, 상대는 assigned 다.
func Start(p StartParams) Date {
	joinedAt := p.Now
	return Date{
		ID:         p.ID,
		CoupleID:   p.CoupleID,
		Theme:      p.Theme,
		Status:     StatusInProgress,
		StartedBy:  p.Starter,
		StartedAt:  p.Now,
		DeadlineAt: p.Now.Add(SubmitWindow),
		Participants: []Participant{
			{UserID: p.Starter, Topic: p.StarterTopic, Status: ParticipantJoined, JoinedAt: &joinedAt},
			{UserID: p.Partner, Topic: p.PartnerTopic, Status: ParticipantAssigned},
		},
	}
}

// EffectiveStatus 는 워커가 아직 돌지 않았더라도 마감이 지났으면 expired 로 본다.
func (d Date) EffectiveStatus(now time.Time) Status {
	if d.Status == StatusInProgress && !now.Before(d.DeadlineAt) {
		return StatusExpired
	}
	return d.Status
}

// IsActiveAt 은 촬영·제출이 가능한 상태(진행 중이고 마감 전)인지 반환한다.
func (d Date) IsActiveAt(now time.Time) bool {
	return d.EffectiveStatus(now) == StatusInProgress
}

// Participant 는 userID 의 참여 정보를 반환한다.
func (d Date) Participant(userID uuid.UUID) (Participant, bool) {
	for _, p := range d.Participants {
		if p.UserID == userID {
			return p, true
		}
	}
	return Participant{}, false
}

// Partner 는 userID 상대의 참여 정보를 반환한다.
func (d Date) Partner(userID uuid.UUID) (Participant, bool) {
	for _, p := range d.Participants {
		if p.UserID != userID {
			return p, true
		}
	}
	return Participant{}, false
}

// HasParticipant 는 userID 가 이 데이트의 참여자인지 반환한다.
func (d Date) HasParticipant(userID uuid.UUID) bool {
	_, ok := d.Participant(userID)
	return ok
}

// CheckCanCapture 는 촬영과 제출의 공통 조건(활성, 미제출, 주제 확인)을 검사한다.
func (d Date) CheckCanCapture(userID uuid.UUID, now time.Time) error {
	me, ok := d.Participant(userID)
	if !ok {
		return apperr.New(apperr.NotFound, "date not found")
	}
	if !d.IsActiveAt(now) {
		return apperr.New(apperr.DateNotActive, "date is not active")
	}
	switch me.Status {
	case ParticipantSubmitted:
		return apperr.New(apperr.AlreadySubmitted, "already submitted")
	case ParticipantAssigned:
		return apperr.New(apperr.DateNotJoined, "topic not confirmed yet")
	}
	return nil
}

// Join 은 assigned 참여자를 joined 로 바꾼다. 이미 joined 이상이면 바뀌지 않는다(멱등).
func (d Date) Join(userID uuid.UUID, now time.Time) (Date, bool, error) {
	me, ok := d.Participant(userID)
	if !ok {
		return d, false, apperr.New(apperr.NotFound, "date not found")
	}
	if me.Status != ParticipantAssigned {
		return d, false, nil
	}
	if !d.IsActiveAt(now) {
		return d, false, apperr.New(apperr.DateNotActive, "date is not active")
	}
	joinedAt := now
	me.Status = ParticipantJoined
	me.JoinedAt = &joinedAt
	return d.withParticipant(me), true, nil
}

// Submit 은 참여자를 submitted 로 바꾸고, 두 사람 모두 제출했으면 데이트를 revealed 로 바꾼다.
// 반환값 revealed 는 이번 제출로 공개됐는지 여부다.
func (d Date) Submit(userID, photoID uuid.UUID, caption *string, now time.Time) (Date, bool, error) {
	if err := d.CheckCanCapture(userID, now); err != nil {
		return d, false, err
	}
	me, _ := d.Participant(userID)
	submittedAt := now
	receiveDeadline := now.Add(ReceiveWindow)
	rep := photoID
	me.Status = ParticipantSubmitted
	me.SubmittedAt = &submittedAt
	me.ReceiveDeadlineAt = &receiveDeadline
	me.RepresentativePhotoID = &rep
	me.Caption = caption

	next := d.withParticipant(me)
	partner, _ := next.Partner(userID)
	if !partner.HasSubmitted() {
		return next, false, nil
	}
	revealedAt := now
	next.Status = StatusRevealed
	next.RevealedAt = &revealedAt
	return next, true, nil
}

// withParticipant 는 참여자 하나를 바꾼 새 Date 를 반환한다.
func (d Date) withParticipant(p Participant) Date {
	next := d
	next.Participants = make([]Participant, len(d.Participants))
	for i, cur := range d.Participants {
		if cur.UserID == p.UserID {
			next.Participants[i] = p
			continue
		}
		next.Participants[i] = cur
	}
	return next
}
