package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
)

type appleLoginRequest struct {
	IdentityToken string  `json:"identity_token" binding:"required"`
	Nickname      *string `json:"nickname"`
}

type googleLoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

type devLoginRequest struct {
	DevID    string  `json:"dev_id" binding:"required"`
	Nickname *string `json:"nickname"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// AuthResponse 는 AuthResponse 스키마다.
type AuthResponse struct {
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresAt  Timestamp    `json:"access_token_expires_at"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt Timestamp    `json:"refresh_token_expires_at"`
	IsNewUser             bool         `json:"is_new_user"`
	User                  UserResponse `json:"user"`
}

// AuthHandler 는 /auth 라우트다.
type AuthHandler struct {
	uc         in.AuthUseCase
	auth       gin.HandlerFunc
	devEnabled bool
	ew         errorWriter
}

// NewAuthHandler 는 AuthHandler 를 생성한다. devEnabled 가 true 일 때만 /auth/dev 를 등록한다.
func NewAuthHandler(uc in.AuthUseCase, logger *slog.Logger, devEnabled bool) *AuthHandler {
	ew := errorWriter{logger: logger}
	return &AuthHandler{uc: uc, auth: Auth(uc, ew), devEnabled: devEnabled, ew: ew}
}

// Register 는 라우트를 등록한다.
func (h *AuthHandler) Register(r gin.IRouter) {
	g := r.Group("/auth")
	g.POST("/apple", h.apple)
	g.POST("/google", h.google)
	if h.devEnabled {
		g.POST("/dev", h.dev)
	}
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.auth, h.logout)
}

func (h *AuthHandler) apple(c *gin.Context) {
	var req appleLoginRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	h.respond(c)(h.uc.LoginApple(c.Request.Context(), req.IdentityToken, req.Nickname))
}

func (h *AuthHandler) google(c *gin.Context) {
	var req googleLoginRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	h.respond(c)(h.uc.LoginGoogle(c.Request.Context(), req.IDToken))
}

func (h *AuthHandler) dev(c *gin.Context) {
	var req devLoginRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	h.respond(c)(h.uc.LoginDev(c.Request.Context(), req.DevID, req.Nickname))
}

func (h *AuthHandler) refresh(c *gin.Context) {
	var req refreshRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	h.respond(c)(h.uc.Refresh(c.Request.Context(), req.RefreshToken))
}

func (h *AuthHandler) logout(c *gin.Context) {
	var req refreshRequest
	if !bindJSON(c, h.ew, &req) {
		return
	}
	if err := h.uc.Logout(c.Request.Context(), userID(c), req.RefreshToken); err != nil {
		h.ew.write(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) respond(c *gin.Context) func(in.AuthResult, error) {
	return func(res in.AuthResult, err error) {
		if err != nil {
			h.ew.write(c, err)
			return
		}
		c.JSON(http.StatusOK, AuthResponse{
			AccessToken: res.AccessToken, AccessTokenExpiresAt: ts(res.AccessTokenExpiresAt),
			RefreshToken: res.RefreshToken, RefreshTokenExpiresAt: ts(res.RefreshTokenExpiresAt),
			IsNewUser: res.IsNewUser, User: toUser(res.User),
		})
	}
}
