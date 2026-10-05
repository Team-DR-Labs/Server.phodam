// Command server 는 애플리케이션 진입점이다.
// 모든 레이어를 아는 유일한 곳으로, 어댑터와 서비스를 조립(DI)하고 서버를 기동한다.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	httpadapter "github.com/Team-DR-Labs/Server.phodam/internal/adapter/in/http"
	"github.com/Team-DR-Labs/Server.phodam/internal/adapter/in/scheduler"
	"github.com/Team-DR-Labs/Server.phodam/internal/adapter/out/persistence/postgres"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/service"
	"github.com/Team-DR-Labs/Server.phodam/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("server exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	gin.SetMode(cfg.App.GinMode)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer func() {
		if err := postgres.Close(db); err != nil {
			logger.Error("close db", slog.Any("error", err))
		}
	}()

	// 아웃바운드 어댑터 → 서비스 → 인바운드 어댑터 순으로 조립한다.
	healthService := service.NewHealthService(postgres.NewHealthRepository(db))
	deps, err := newDeps(ctx, cfg, db, logger)
	if err != nil {
		return err
	}
	router := httpadapter.NewRouter(logger,
		httpadapter.NewHealthHandler(healthService, logger),
		newAPI(cfg, deps, logger),
	)

	// 워커는 HTTP 서버와 같은 수명이다. 종료 신호를 받으면 진행 중인 주기를 마친 뒤 멈춘다.
	var workers sync.WaitGroup
	worker := scheduler.NewWorker(service.NewMaintenanceService(deps), cfg.Worker.Interval, logger)
	workers.Go(func() { worker.Run(ctx) })
	defer func() {
		stop()
		workers.Wait()
	}()

	return serve(ctx, cfg.App, router, logger)
}

// serve 는 HTTP 서버를 띄우고 ctx 가 끝나면 ShutdownTimeout 안에 정상 종료한다.
func serve(ctx context.Context, cfg config.AppConfig, router http.Handler, logger *slog.Logger) error {
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server started", slog.String("addr", srv.Addr), slog.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}
