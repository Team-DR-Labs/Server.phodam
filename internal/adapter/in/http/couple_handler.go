package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
)

type joinCoupleRequest struct {
	Code string `json:"code" binding:"required"`
}

// InviteResponse 는 Invite 스키마다.
type InviteResponse struct {
	Code      string    `json:"code"`
	ExpiresAt Timestamp `json:"expires_at"`
}

// CoupleHandler 는 /couple 라우트다.
type CoupleHandler struct {
	uc   in.CoupleUseCase
	auth gin.HandlerFunc
	ew   errorWriter
}

// NewCoupleHandler 는 CoupleHandler 를 생성한다.
func NewCoupleHandler(uc in.CoupleUseCase, auth in.AuthUseCase, logger *slog.Logger) *CoupleHandler {
	ew := errorWriter{logger: logger}
	return &CoupleHandler{uc: uc, auth: Auth(auth, ew), ew: ew}
}

// Register 는 라우트를 등록한다.
func (h *CoupleHandler) Register(r gin.IRouter) {
	g := r.Group("/couple", h.auth)
	g.GET("", h.get)
	g.POST("/invites", h.invite)
	g.POST("/join", h.join)
}

func (h *CoupleHandler) get(c *gin.Context) {
	cv, err := h.uc.Get(c.Request.Context(), userID(c))
	if err != nil {
		h.ew.write(c, err)
		return
	}
	c.JSON(http.StatusOK, toCouple(cv))
}

func (h *CoupleHandler) invite(c *gin.Context) {
	inv, err := h.uc.CreateInvite(c.Request.Context(), userID(c))
	if err != nil {
		h.ew.write(c, err)
		return
	}
	c.JSON(http.StatusCreated, InviteResponse{Code: inv.Code, ExpiresAt: ts(inv.ExpiresAt)})
}

func (h *CoupleHandler) join(c *gin.Context) {
	var req joinCoupleRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	cv, err := h.uc.Join(c.Request.Context(), userID(c), req.Code)
	if err != nil {
		h.ew.write(c, err)
		return
	}
	c.JSON(http.StatusOK, toCouple(cv))
}
