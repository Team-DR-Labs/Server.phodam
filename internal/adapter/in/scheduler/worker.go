// Package scheduler 는 주기 작업을 실행하는 인바운드 어댑터다 (서버 프로세스 내부 goroutine).
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
)

// Worker 는 interval 마다 MaintenanceUseCase.RunOnce 를 호출한다.
type Worker struct {
	uc       in.MaintenanceUseCase
	interval time.Duration
	logger   *slog.Logger
}

// NewWorker 는 Worker 를 생성한다.
func NewWorker(uc in.MaintenanceUseCase, interval time.Duration, logger *slog.Logger) *Worker {
	return &Worker{uc: uc, interval: interval, logger: logger}
}

// Run 은 ctx 가 끝날 때까지 주기 작업을 돌린다. 기동 직후 한 번 실행한다.
// 실행 중인 주기는 끝까지 마친 뒤 반환한다 (graceful shutdown).
func (w *Worker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "worker started", slog.Duration("interval", w.interval))
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		w.uc.RunOnce(context.WithoutCancel(ctx))
		select {
		case <-ctx.Done():
			w.logger.Info("worker stopped")
			return
		case <-ticker.C:
		}
	}
}
