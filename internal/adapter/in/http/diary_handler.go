package http

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
)

// DiaryListItemResponse 는 DiaryListItem 스키마다.
type DiaryListItemResponse struct {
	DateID                string           `json:"date_id"`
	LocalDate             string           `json:"local_date"`
	Theme                 ThemeResponse    `json:"theme"`
	Visibility            diary.Visibility `json:"visibility"`
	StartedAt             Timestamp        `json:"started_at"`
	ThumbnailURL          string           `json:"thumbnail_url"`
	ThumbnailURLExpiresAt Timestamp        `json:"thumbnail_url_expires_at"`
}

// DiaryListResponse 는 DiaryList 스키마다.
type DiaryListResponse struct {
	Items      []DiaryListItemResponse `json:"items"`
	NextCursor *string                 `json:"next_cursor"`
}

// DiaryEntryResponse 는 DiaryEntry 스키마다.
type DiaryEntryResponse struct {
	Author            UserResponse  `json:"author"`
	IsMe              bool          `json:"is_me"`
	Topic             TopicResponse `json:"topic"`
	Caption           *string       `json:"caption"`
	PhotoURL          string        `json:"photo_url"`
	PhotoURLExpiresAt Timestamp     `json:"photo_url_expires_at"`
	SubmittedAt       Timestamp     `json:"submitted_at"`
}

// DiaryDetailResponse 는 DiaryDetail 스키마다.
type DiaryDetailResponse struct {
	DateID     string               `json:"date_id"`
	LocalDate  string               `json:"local_date"`
	Theme      ThemeResponse        `json:"theme"`
	Visibility diary.Visibility     `json:"visibility"`
	StartedAt  Timestamp            `json:"started_at"`
	Entries    []DiaryEntryResponse `json:"entries"`
}

// DiaryHandler 는 /diaries 라우트다.
type DiaryHandler struct {
	uc   in.DiaryUseCase
	auth gin.HandlerFunc
	ew   errorWriter
}

// NewDiaryHandler 는 DiaryHandler 를 생성한다.
func NewDiaryHandler(uc in.DiaryUseCase, auth in.AuthUseCase, logger *slog.Logger) *DiaryHandler {
	ew := errorWriter{logger: logger}
	return &DiaryHandler{uc: uc, auth: Auth(auth, ew), ew: ew}
}

// Register 는 라우트를 등록한다.
func (h *DiaryHandler) Register(r gin.IRouter) {
	g := r.Group("/diaries", h.auth)
	g.GET("", h.list)
	g.GET("/:dateId", h.get)
}

func (h *DiaryHandler) list(c *gin.Context) {
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			h.ew.validation(c, "limit must be an integer")
			return
		}
		limit = n
	}
	page, err := h.uc.List(c.Request.Context(), userID(c), c.Query("cursor"), limit)
	writeReply(c, h.ew, http.StatusOK, err, func() any { return toDiaryList(page) })
}

func (h *DiaryHandler) get(c *gin.Context) {
	id, ok := pathUUID(c, h.ew, "dateId")
	if !ok {
		return
	}
	d, err := h.uc.Get(c.Request.Context(), userID(c), id)
	writeReply(c, h.ew, http.StatusOK, err, func() any { return toDiaryDetail(d) })
}

func toDiaryList(p diary.Page) DiaryListResponse {
	items := make([]DiaryListItemResponse, 0, len(p.Items))
	for _, it := range p.Items {
		items = append(items, DiaryListItemResponse{
			DateID: it.DateID.String(), LocalDate: it.LocalDate, Theme: toTheme(it.Theme),
			Visibility: it.Visibility, StartedAt: ts(it.StartedAt),
			ThumbnailURL: it.Thumbnail.URL, ThumbnailURLExpiresAt: ts(it.Thumbnail.ExpiresAt),
		})
	}
	return DiaryListResponse{Items: items, NextCursor: p.NextCursor}
}

func toDiaryDetail(d diary.Detail) DiaryDetailResponse {
	entries := make([]DiaryEntryResponse, 0, len(d.Entries))
	for _, e := range d.Entries {
		entries = append(entries, DiaryEntryResponse{
			Author: toUser(e.Author), IsMe: e.IsMe, Topic: toTopic(e.Topic), Caption: e.Caption,
			PhotoURL: e.Photo.URL, PhotoURLExpiresAt: ts(e.Photo.ExpiresAt), SubmittedAt: ts(e.SubmittedAt),
		})
	}
	return DiaryDetailResponse{
		DateID: d.DateID.String(), LocalDate: d.LocalDate, Theme: toTheme(d.Theme),
		Visibility: d.Visibility, StartedAt: ts(d.StartedAt), Entries: entries,
	}
}
