package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
)

type submitRequest struct {
	PhotoID string  `json:"photo_id" binding:"required"`
	Caption *string `json:"caption"`
}

// DateHandler 는 /dates 라우트다 (데이트·촬영·제출·수령).
type DateHandler struct {
	dates  in.DateUseCase
	photos in.PhotoUseCase
	auth   gin.HandlerFunc
	ew     errorWriter
}

// NewDateHandler 는 DateHandler 를 생성한다.
func NewDateHandler(dates in.DateUseCase, photos in.PhotoUseCase, auth in.AuthUseCase, logger *slog.Logger) *DateHandler {
	ew := errorWriter{logger: logger}
	return &DateHandler{dates: dates, photos: photos, auth: Auth(auth, ew), ew: ew}
}

// Register 는 라우트를 등록한다.
func (h *DateHandler) Register(r gin.IRouter) {
	g := r.Group("/dates", h.auth)
	g.POST("", h.start)
	g.GET("/current", h.current)
	g.GET("/:dateId", h.get)
	g.POST("/:dateId/join", h.join)
	g.POST("/:dateId/shots", h.shot)
	g.GET("/:dateId/my-photos", h.myPhotos)
	g.POST("/:dateId/submit", h.submit)
	g.GET("/:dateId/receivable", h.receivable)
}

func (h *DateHandler) start(c *gin.Context) {
	v, err := h.dates.Start(c.Request.Context(), userID(c))
	if err != nil {
		h.ew.write(c, err)
		return
	}
	c.JSON(http.StatusCreated, toDateView(v))
}

func (h *DateHandler) current(c *gin.Context) {
	v, err := h.dates.Current(c.Request.Context(), userID(c))
	if err != nil {
		h.ew.write(c, err)
		return
	}
	if v == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, toDateView(*v))
}

func (h *DateHandler) get(c *gin.Context) {
	h.withDate(c, func(id uuid.UUID) {
		v, err := h.dates.Get(c.Request.Context(), userID(c), id)
		h.reply(c, http.StatusOK, err, func() any { return toDateView(v) })
	})
}

func (h *DateHandler) join(c *gin.Context) {
	h.withDate(c, func(id uuid.UUID) {
		v, err := h.dates.Join(c.Request.Context(), userID(c), id)
		h.reply(c, http.StatusOK, err, func() any { return toDateView(v) })
	})
}

func (h *DateHandler) shot(c *gin.Context) {
	h.withDate(c, func(id uuid.UUID) {
		res, err := h.photos.ReserveShot(c.Request.Context(), userID(c), id)
		h.reply(c, http.StatusCreated, err, func() any {
			return ShotReservationResponse{Photo: toPhoto(res.Photo), Upload: toUploadTarget(res.Upload), FilmBalance: res.FilmBalance}
		})
	})
}

func (h *DateHandler) myPhotos(c *gin.Context) {
	h.withDate(c, func(id uuid.UUID) {
		items, err := h.photos.ListMine(c.Request.Context(), userID(c), id)
		h.reply(c, http.StatusOK, err, func() any { return PhotoListResponse{Items: toPhotosWithURL(items)} })
	})
}

func (h *DateHandler) submit(c *gin.Context) {
	h.withDate(c, func(id uuid.UUID) {
		var req submitRequest
		if !bindJSON(c, h.ew, &req) {
			return
		}
		photoID, err := uuid.Parse(req.PhotoID)
		if err != nil {
			h.ew.validation(c, "photo_id must be a uuid")
			return
		}
		v, err := h.dates.Submit(c.Request.Context(), userID(c), id, photoID, req.Caption)
		h.reply(c, http.StatusOK, err, func() any { return toDateView(v) })
	})
}

func (h *DateHandler) receivable(c *gin.Context) {
	h.withDate(c, func(id uuid.UUID) {
		res, err := h.photos.Receivable(c.Request.Context(), userID(c), id)
		h.reply(c, http.StatusOK, err, func() any {
			return ReceivableResponse{ReceiveDeadlineAt: ts(res.ReceiveDeadlineAt), Items: toPhotosWithURL(res.Items)}
		})
	})
}

func (h *DateHandler) withDate(c *gin.Context, fn func(uuid.UUID)) {
	if id, ok := pathUUID(c, h.ew, "dateId"); ok {
		fn(id)
	}
}

// reply 는 에러면 에러 응답을, 아니면 body() 를 status 로 쓴다.
func (h *DateHandler) reply(c *gin.Context, status int, err error, body func() any) {
	writeReply(c, h.ew, status, err, body)
}

func writeReply(c *gin.Context, ew errorWriter, status int, err error, body func() any) {
	if err != nil {
		ew.write(c, err)
		return
	}
	c.JSON(status, body())
}
