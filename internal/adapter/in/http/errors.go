package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
)

// statusByCode 는 정책 §13 의 에러 코드 → HTTP 상태 매핑이다. 단일 매퍼로 유지한다.
var statusByCode = map[apperr.Code]int{
	apperr.ValidationFailed:        http.StatusBadRequest,
	apperr.Unauthorized:            http.StatusUnauthorized,
	apperr.AuthInvalidIDToken:      http.StatusUnauthorized,
	apperr.AuthInvalidRefreshToken: http.StatusUnauthorized,
	apperr.Forbidden:               http.StatusForbidden,
	apperr.CoupleRequired:          http.StatusForbidden,
	apperr.NotFound:                http.StatusNotFound,
	apperr.CoupleAlreadyConnected:  http.StatusConflict,
	apperr.InviteInvalid:           http.StatusBadRequest,
	apperr.DateAlreadyInProgress:   http.StatusConflict,
	apperr.DateNotActive:           http.StatusConflict,
	apperr.DateNotJoined:           http.StatusConflict,
	apperr.AlreadySubmitted:        http.StatusConflict,
	apperr.FilmExhausted:           http.StatusConflict,
	apperr.PhotoNotUploaded:        http.StatusConflict,
	apperr.PhotoInvalid:            http.StatusBadRequest,
	apperr.ReceiveNotAvailable:     http.StatusConflict,
	apperr.Internal:                http.StatusInternalServerError,
}

// errorWriter 는 에러를 ErrorResponse 로 쓴다. 도메인 에러가 아니면 내용을 로그에만 남기고 INTERNAL_ERROR 로 응답한다.
type errorWriter struct {
	logger *slog.Logger
}

func (w errorWriter) write(c *gin.Context, err error) {
	if e, ok := apperr.As(err); ok {
		status, known := statusByCode[e.Code]
		if known {
			c.AbortWithStatusJSON(status, ErrorResponse{Code: string(e.Code), Message: e.Message})
			return
		}
	}
	w.logger.ErrorContext(c.Request.Context(), "request failed",
		slog.String("path", c.FullPath()), slog.Any("error", err))
	c.AbortWithStatusJSON(http.StatusInternalServerError,
		ErrorResponse{Code: string(apperr.Internal), Message: "internal server error"})
}

func (w errorWriter) validation(c *gin.Context, msg string) {
	w.write(c, apperr.New(apperr.ValidationFailed, msg))
}

func (w errorWriter) notFound(c *gin.Context) {
	w.write(c, apperr.New(apperr.NotFound, "resource not found"))
}
