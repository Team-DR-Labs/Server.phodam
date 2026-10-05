package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
)

type filmGrantRequest struct {
	Shots  int     `json:"shots" binding:"required"`
	Reason *string `json:"reason"`
}

// FilmGrantResponse 는 FilmGrantResponse 스키마다.
type FilmGrantResponse struct {
	UserID      string `json:"user_id"`
	FilmBalance int    `json:"film_balance"`
}

// AdminHandler 는 /admin 라우트다. X-Admin-Key 로 보호한다.
type AdminHandler struct {
	uc  in.AdminUseCase
	key gin.HandlerFunc
	ew  errorWriter
}

// NewAdminHandler 는 AdminHandler 를 생성한다. apiKey 가 비어 있으면 호출하지 않는다 (라우트 비활성).
func NewAdminHandler(uc in.AdminUseCase, apiKey string, logger *slog.Logger) *AdminHandler {
	ew := errorWriter{logger: logger}
	return &AdminHandler{uc: uc, key: AdminKey(apiKey, ew), ew: ew}
}

// Register 는 라우트를 등록한다.
func (h *AdminHandler) Register(r gin.IRouter) {
	g := r.Group("/admin", h.key)
	g.POST("/users/:userId/film-grants", h.grant)
}

func (h *AdminHandler) grant(c *gin.Context) {
	id, ok := pathUUID(c, h.ew, "userId")
	if !ok {
		return
	}
	var req filmGrantRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	balance, err := h.uc.GrantFilm(c.Request.Context(), id, req.Shots, req.Reason)
	writeReply(c, h.ew, http.StatusOK, err, func() any {
		return FilmGrantResponse{UserID: id.String(), FilmBalance: balance}
	})
}
