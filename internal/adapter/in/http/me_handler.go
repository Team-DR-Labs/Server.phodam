package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
)

type updateMeRequest struct {
	Nickname string `json:"nickname" binding:"required"`
}

type deviceRequest struct {
	FCMToken string `json:"fcm_token" binding:"required"`
	Platform string `json:"platform" binding:"required"`
}

// MeResponse 는 Me 스키마다.
type MeResponse struct {
	User        UserResponse      `json:"user"`
	FilmBalance int               `json:"film_balance"`
	Couple      *CoupleResponse   `json:"couple"`
	CurrentDate *DateViewResponse `json:"current_date"`
}

func toMe(m in.Me) MeResponse {
	res := MeResponse{User: toUser(m.User), FilmBalance: m.FilmBalance, CurrentDate: toDateViewPtr(m.CurrentDate)}
	if m.Couple != nil {
		c := toCouple(*m.Couple)
		res.Couple = &c
	}
	return res
}

// MeHandler 는 /me 라우트다.
type MeHandler struct {
	uc   in.MeUseCase
	auth gin.HandlerFunc
	ew   errorWriter
}

// NewMeHandler 는 MeHandler 를 생성한다.
func NewMeHandler(uc in.MeUseCase, auth in.AuthUseCase, logger *slog.Logger) *MeHandler {
	ew := errorWriter{logger: logger}
	return &MeHandler{uc: uc, auth: Auth(auth, ew), ew: ew}
}

// Register 는 라우트를 등록한다.
func (h *MeHandler) Register(r gin.IRouter) {
	g := r.Group("/me", h.auth)
	g.GET("", h.get)
	g.PATCH("", h.update)
	g.PUT("/devices", h.device)
}

func (h *MeHandler) get(c *gin.Context) {
	me, err := h.uc.Get(c.Request.Context(), userID(c))
	if err != nil {
		h.ew.write(c, err)
		return
	}
	c.JSON(http.StatusOK, toMe(me))
}

func (h *MeHandler) update(c *gin.Context) {
	var req updateMeRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	me, err := h.uc.UpdateNickname(c.Request.Context(), userID(c), req.Nickname)
	if err != nil {
		h.ew.write(c, err)
		return
	}
	c.JSON(http.StatusOK, toMe(me))
}

func (h *MeHandler) device(c *gin.Context) {
	var req deviceRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	if err := h.uc.RegisterDevice(c.Request.Context(), userID(c), req.FCMToken, req.Platform); err != nil {
		h.ew.write(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
