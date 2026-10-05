package http

import (
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
)

// PhotoResponse 는 Photo 스키마다.
type PhotoResponse struct {
	ID               string       `json:"id"`
	DateID           string       `json:"date_id"`
	Status           photo.Status `json:"status"`
	IsRepresentative bool         `json:"is_representative"`
	CreatedAt        Timestamp    `json:"created_at"`
	UploadedAt       *Timestamp   `json:"uploaded_at"`
	ReceivedAt       *Timestamp   `json:"received_at"`
}

// PhotoWithURLResponse 는 PhotoWithUrl 스키마다.
type PhotoWithURLResponse struct {
	PhotoResponse
	URL          string    `json:"url"`
	URLExpiresAt Timestamp `json:"url_expires_at"`
}

// PhotoListResponse 는 PhotoList 스키마다.
type PhotoListResponse struct {
	Items []PhotoWithURLResponse `json:"items"`
}

// UploadTargetResponse 는 UploadTarget 스키마다.
type UploadTargetResponse struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt Timestamp         `json:"expires_at"`
}

// ShotReservationResponse 는 ShotReservation 스키마다.
type ShotReservationResponse struct {
	Photo       PhotoResponse        `json:"photo"`
	Upload      UploadTargetResponse `json:"upload"`
	FilmBalance int                  `json:"film_balance"`
}

// ReceivableResponse 는 Receivable 스키마다.
type ReceivableResponse struct {
	ReceiveDeadlineAt Timestamp              `json:"receive_deadline_at"`
	Items             []PhotoWithURLResponse `json:"items"`
}

func toPhoto(p photo.Photo) PhotoResponse {
	return PhotoResponse{
		ID: p.ID.String(), DateID: p.DateID.String(), Status: p.Status, IsRepresentative: p.IsRepresentative,
		CreatedAt: ts(p.CreatedAt), UploadedAt: tsPtr(p.UploadedAt), ReceivedAt: tsPtr(p.ReceivedAt),
	}
}

func toPhotosWithURL(items []in.PhotoWithURL) []PhotoWithURLResponse {
	res := make([]PhotoWithURLResponse, 0, len(items))
	for _, it := range items {
		res = append(res, PhotoWithURLResponse{PhotoResponse: toPhoto(it.Photo), URL: it.URL, URLExpiresAt: ts(it.URLExpiresAt)})
	}
	return res
}

func toUploadTarget(u in.UploadTarget) UploadTargetResponse {
	return UploadTargetResponse{URL: u.URL, Method: u.Method, Headers: u.Headers, ExpiresAt: ts(u.ExpiresAt)}
}
