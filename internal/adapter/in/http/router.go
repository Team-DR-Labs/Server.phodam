// Package http 는 Gin 기반 HTTP 인바운드 어댑터다.
package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 는 라우트를 등록하는 HTTP 핸들러다. 새 도메인 핸들러는 이 인터페이스를 구현한다.
type Handler interface {
	Register(r gin.IRouter)
}

// NewRouter 는 공통 미들웨어를 적용한 gin.Engine 을 만들고 핸들러 라우트를 등록한다.
func NewRouter(logger *slog.Logger, handlers ...Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.CustomRecovery(recoveryHandler(logger)), requestLogger(logger))
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, ErrorResponse{Code: "NOT_FOUND", Message: "resource not found"})
	})

	for _, h := range handlers {
		h.Register(r)
	}
	return r
}

// Group 은 prefix 아래에 여러 핸들러를 등록하는 Handler 다 (예: /v1).
type Group struct {
	prefix   string
	handlers []Handler
}

// NewGroup 은 Group 을 생성한다.
func NewGroup(prefix string, handlers ...Handler) Group {
	return Group{prefix: prefix, handlers: handlers}
}

// Register 는 prefix 그룹에 핸들러 라우트를 등록한다.
func (g Group) Register(r gin.IRouter) {
	sub := r.Group(g.prefix)
	for _, h := range g.handlers {
		h.Register(sub)
	}
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logger.InfoContext(c.Request.Context(), "http request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}

func recoveryHandler(logger *slog.Logger) gin.RecoveryFunc {
	return func(c *gin.Context, err any) {
		logger.ErrorContext(c.Request.Context(), "panic recovered", slog.Any("error", err))
		c.AbortWithStatusJSON(http.StatusInternalServerError,
			ErrorResponse{Code: "INTERNAL_ERROR", Message: "internal server error"})
	}
}
