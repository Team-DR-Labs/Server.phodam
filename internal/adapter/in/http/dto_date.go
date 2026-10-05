package http

import "github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"

// ThemeResponse 는 Theme 스키마다.
type ThemeResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// TopicResponse 는 Topic 스키마다.
type TopicResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// MyParticipationResponse 는 MyParticipation 스키마다.
type MyParticipationResponse struct {
	Status            dating.ParticipantStatus `json:"status"`
	Topic             *TopicResponse           `json:"topic"`
	JoinedAt          *Timestamp               `json:"joined_at"`
	SubmittedAt       *Timestamp               `json:"submitted_at"`
	ReceiveDeadlineAt *Timestamp               `json:"receive_deadline_at"`
	ShotCount         int                      `json:"shot_count"`
}

// PartnerParticipationResponse 는 PartnerParticipation 스키마다.
type PartnerParticipationResponse struct {
	User   UserResponse             `json:"user"`
	Status dating.ParticipantStatus `json:"status"`
	Topic  *TopicResponse           `json:"topic"`
}

// DateViewResponse 는 DateView 스키마다.
type DateViewResponse struct {
	ID          string                       `json:"id"`
	Status      dating.Status                `json:"status"`
	Theme       ThemeResponse                `json:"theme"`
	StartedByMe bool                         `json:"started_by_me"`
	StartedAt   Timestamp                    `json:"started_at"`
	DeadlineAt  Timestamp                    `json:"deadline_at"`
	RevealedAt  *Timestamp                   `json:"revealed_at"`
	Me          MyParticipationResponse      `json:"me"`
	Partner     PartnerParticipationResponse `json:"partner"`
}

func toTheme(t dating.Theme) ThemeResponse { return ThemeResponse{ID: t.ID.String(), Title: t.Title} }

func toTopic(t dating.Topic) TopicResponse { return TopicResponse{ID: t.ID.String(), Title: t.Title} }

func toTopicPtr(t *dating.Topic) *TopicResponse {
	if t == nil {
		return nil
	}
	v := toTopic(*t)
	return &v
}

func toDateView(v dating.View) DateViewResponse {
	return DateViewResponse{
		ID: v.ID.String(), Status: v.Status, Theme: toTheme(v.Theme), StartedByMe: v.StartedByMe,
		StartedAt: ts(v.StartedAt), DeadlineAt: ts(v.DeadlineAt), RevealedAt: tsPtr(v.RevealedAt),
		Me: MyParticipationResponse{
			Status: v.Me.Status, Topic: toTopicPtr(v.Me.Topic), JoinedAt: tsPtr(v.Me.JoinedAt),
			SubmittedAt: tsPtr(v.Me.SubmittedAt), ReceiveDeadlineAt: tsPtr(v.Me.ReceiveDeadlineAt),
			ShotCount: v.Me.ShotCount,
		},
		Partner: PartnerParticipationResponse{
			User: toUser(v.Partner.User), Status: v.Partner.Status, Topic: toTopicPtr(v.Partner.Topic),
		},
	}
}

func toDateViewPtr(v *dating.View) *DateViewResponse {
	if v == nil {
		return nil
	}
	r := toDateView(*v)
	return &r
}
