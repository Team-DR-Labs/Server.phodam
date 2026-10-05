package http

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/health"
)

// HealthHandler 는 상태 점검 HTTP 인바운드 어댑터다.
type HealthHandler struct {
	useCase in.HealthUseCase
	logger  *slog.Logger
}

// NewHealthHandler 는 HealthHandler 를 생성한다.
func NewHealthHandler(useCase in.HealthUseCase, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{useCase: useCase, logger: logger}
}

// Register 는 상태 점검 라우트를 등록한다.
func (h *HealthHandler) Register(r gin.IRouter) {
	g := r.Group("/health")
	g.GET("/live", h.live)
	g.GET("/ready", h.ready)
}

func (h *HealthHandler) live(c *gin.Context) {
	h.write(c, h.useCase.Liveness(c.Request.Context()))
}

func (h *HealthHandler) ready(c *gin.Context) {
	h.write(c, h.useCase.Readiness(c.Request.Context()))
}

func (h *HealthHandler) write(c *gin.Context, report health.Report) {
	if !report.IsUp() {
		for _, comp := range report.Components {
			if comp.Status != health.StatusUp {
				h.logger.WarnContext(c.Request.Context(), "health check failed",
					slog.String("component", comp.Name), slog.String("error", comp.Error))
			}
		}
		c.JSON(http.StatusServiceUnavailable, toHealthResponse(report))
		return
	}
	c.JSON(http.StatusOK, toHealthResponse(report))
}
