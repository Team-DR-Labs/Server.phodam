package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/health"
)

type fakeHealthUseCase struct {
	ready health.Report
}

func (f fakeHealthUseCase) Liveness(context.Context) health.Report  { return health.NewReport() }
func (f fakeHealthUseCase) Readiness(context.Context) health.Report { return f.ready }

func newTestRouter(uc fakeHealthUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(logger, NewHealthHandler(uc, logger))
}

func TestHealthHandler(t *testing.T) {
	down := health.NewReport(health.Component{Name: "database", Status: health.StatusDown, Error: "secret detail"})
	up := health.NewReport(health.Component{Name: "database", Status: health.StatusUp})

	tests := []struct {
		name       string
		path       string
		ready      health.Report
		wantCode   int
		wantStatus health.Status
	}{
		{name: "live", path: "/health/live", ready: down, wantCode: http.StatusOK, wantStatus: health.StatusUp},
		{name: "ready up", path: "/health/ready", ready: up, wantCode: http.StatusOK, wantStatus: health.StatusUp},
		{name: "ready down", path: "/health/ready", ready: down, wantCode: http.StatusServiceUnavailable, wantStatus: health.StatusDown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			newTestRouter(fakeHealthUseCase{ready: tt.ready}).ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tt.wantCode)
			}
			var body HealthResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Status != tt.wantStatus {
				t.Fatalf("status = %s, want %s", body.Status, tt.wantStatus)
			}
			if strings.Contains(rec.Body.String(), "secret detail") {
				t.Fatal("internal error detail must not be exposed")
			}
		})
	}
}

func TestRouter_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)

	newTestRouter(fakeHealthUseCase{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("code = %d, want 404", rec.Code)
	}
}
