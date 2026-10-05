package http

import (
	"crypto/subtle"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
)

const ctxUserID = "phodam.user_id"

// Auth 는 Bearer 액세스 토큰을 검증하고 사용자 ID 를 컨텍스트에 싣는 미들웨어다.
func Auth(auth in.AuthUseCase, ew errorWriter) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || strings.TrimSpace(raw) == "" {
			ew.write(c, apperr.New(apperr.Unauthorized, "missing access token"))
			return
		}
		id, err := auth.Authenticate(c.Request.Context(), strings.TrimSpace(raw))
		if err != nil {
			ew.write(c, err)
			return
		}
		c.Set(ctxUserID, id)
		c.Next()
	}
}

// AdminKey 는 X-Admin-Key 헤더를 상수 시간 비교로 확인한다.
func AdminKey(key string, ew errorWriter) gin.HandlerFunc {
	want := []byte(key)
	return func(c *gin.Context) {
		got := []byte(c.GetHeader("X-Admin-Key"))
		if len(want) == 0 || subtle.ConstantTimeCompare(got, want) != 1 {
			ew.write(c, apperr.New(apperr.Forbidden, "invalid admin key"))
			return
		}
		c.Next()
	}
}

// userID 는 Auth 미들웨어가 넣은 사용자 ID 다.
func userID(c *gin.Context) uuid.UUID {
	id, _ := c.Get(ctxUserID)
	uid, _ := id.(uuid.UUID)
	return uid
}

// pathUUID 는 경로 파라미터를 UUID 로 읽는다. 형식이 틀리면 NOT_FOUND 로 응답하고 false 를 반환한다.
func pathUUID(c *gin.Context, ew errorWriter, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		ew.notFound(c)
		return uuid.Nil, false
	}
	return id, true
}

// bindJSON 은 요청 본문을 읽는다. 실패하면 VALIDATION_FAILED 로 응답하고 false 를 반환한다.
func bindJSON(c *gin.Context, ew errorWriter, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		ew.validation(c, "invalid request body")
		return false
	}
	return true
}
