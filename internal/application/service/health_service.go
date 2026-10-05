// Package service 는 인바운드 포트(유스케이스)를 구현한다.
// 아웃바운드 포트 인터페이스에만 의존하고 어댑터 구현은 알지 못한다.
package service

import (
	"context"
	"time"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/health"
)

const (
	componentDatabase   = "database"
	defaultCheckTimeout = 2 * time.Second
)

// HealthService 는 HealthUseCase 구현체다.
type HealthService struct {
	db      out.DatabasePinger
	timeout time.Duration
}

var _ in.HealthUseCase = (*HealthService)(nil)

// NewHealthService 는 HealthService 를 생성한다.
func NewHealthService(db out.DatabasePinger) *HealthService {
	return &HealthService{db: db, timeout: defaultCheckTimeout}
}

// Liveness 는 항상 up 을 반환한다.
func (s *HealthService) Liveness(_ context.Context) health.Report {
	return health.NewReport()
}

// Readiness 는 DB 연결을 확인한다.
func (s *HealthService) Readiness(ctx context.Context) health.Report {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	return health.NewReport(s.checkDatabase(ctx))
}

func (s *HealthService) checkDatabase(ctx context.Context) health.Component {
	if err := s.db.Ping(ctx); err != nil {
		return health.Component{Name: componentDatabase, Status: health.StatusDown, Error: err.Error()}
	}
	return health.Component{Name: componentDatabase, Status: health.StatusUp}
}
