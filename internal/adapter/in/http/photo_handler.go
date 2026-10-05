package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
)

// PhotoHandler 는 /photos 라우트다.
type PhotoHandler struct {
	uc   in.PhotoUseCase
	auth gin.HandlerFunc
	ew   errorWriter
}

// NewPhotoHandler 는 PhotoHandler 를 생성한다.
func NewPhotoHandler(uc in.PhotoUseCase, auth in.AuthUseCase, logger *slog.Logger) *PhotoHandler {
	ew := errorWriter{logger: logger}
	return &PhotoHandler{uc: uc, auth: Auth(auth, ew), ew: ew}
}

// Register 는 라우트를 등록한다.
func (h *PhotoHandler) Register(r gin.IRouter) {
	g := r.Group("/photos", h.auth)
	g.POST("/:photoId/upload-url", h.uploadURL)
	g.POST("/:photoId/complete", h.complete)
	g.POST("/:photoId/received", h.received)
}

func (h *PhotoHandler) uploadURL(c *gin.Context) {
	id, ok := pathUUID(c, h.ew, "photoId")
	if !ok {
		return
	}
	t, err := h.uc.ReissueUploadURL(c.Request.Context(), userID(c), id)
	writeReply(c, h.ew, http.StatusOK, err, func() any { return toUploadTarget(t) })
}

func (h *PhotoHandler) complete(c *gin.Context) {
	id, ok := pathUUID(c, h.ew, "photoId")
	if !ok {
		return
	}
	p, err := h.uc.CompleteUpload(c.Request.Context(), userID(c), id)
	writeReply(c, h.ew, http.StatusOK, err, func() any { return toPhoto(p) })
}

func (h *PhotoHandler) received(c *gin.Context) {
	id, ok := pathUUID(c, h.ew, "photoId")
	if !ok {
		return
	}
	p, err := h.uc.AckReceived(c.Request.Context(), userID(c), id)
	writeReply(c, h.ew, http.StatusOK, err, func() any { return toPhoto(p) })
}
