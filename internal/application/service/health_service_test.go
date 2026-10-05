package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/health"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealthService_Liveness(t *testing.T) {
	svc := NewHealthService(fakePinger{err: errors.New("db down")})

	report := svc.Liveness(context.Background())

	if !report.IsUp() {
		t.Fatalf("liveness should be up regardless of db, got %s", report.Status)
	}
}

func TestHealthService_Readiness(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus health.Status
	}{
		{name: "db up", pingErr: nil, wantStatus: health.StatusUp},
		{name: "db down", pingErr: errors.New("connection refused"), wantStatus: health.StatusDown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewHealthService(fakePinger{err: tt.pingErr})

			report := svc.Readiness(context.Background())

			if report.Status != tt.wantStatus {
				t.Fatalf("status = %s, want %s", report.Status, tt.wantStatus)
			}
			if len(report.Components) != 1 || report.Components[0].Name != componentDatabase {
				t.Fatalf("unexpected components: %+v", report.Components)
			}
			if tt.pingErr != nil && report.Components[0].Error == "" {
				t.Fatal("expected error message on down component")
			}
		})
	}
}
