// Package in 은 외부(HTTP 등)에서 애플리케이션을 호출하는 인바운드 포트를 정의한다.
package in

import (
	"context"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/health"
)

// HealthUseCase 는 서비스 상태 점검 유스케이스다.
type HealthUseCase interface {
	// Liveness 는 프로세스가 살아 있는지만 확인한다. 외부 의존성은 보지 않는다.
	Liveness(ctx context.Context) health.Report
	// Readiness 는 트래픽을 받을 준비(DB 연결 등)가 됐는지 확인한다.
	Readiness(ctx context.Context) health.Report
}
