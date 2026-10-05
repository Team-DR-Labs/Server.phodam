package main

import (
	"context"
	"log/slog"

	"gorm.io/gorm"

	httpadapter "github.com/Team-DR-Labs/Server.phodam/internal/adapter/in/http"
	"github.com/Team-DR-Labs/Server.phodam/internal/adapter/out/clock"
	"github.com/Team-DR-Labs/Server.phodam/internal/adapter/out/idtoken"
	"github.com/Team-DR-Labs/Server.phodam/internal/adapter/out/persistence/postgres"
	"github.com/Team-DR-Labs/Server.phodam/internal/adapter/out/push"
	"github.com/Team-DR-Labs/Server.phodam/internal/adapter/out/storage"
	"github.com/Team-DR-Labs/Server.phodam/internal/adapter/out/token"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/service"
	"github.com/Team-DR-Labs/Server.phodam/internal/config"
)

// newDeps 는 아웃바운드 어댑터를 만들어 서비스 의존성으로 묶는다.
func newDeps(ctx context.Context, cfg config.Config, db *gorm.DB, logger *slog.Logger) (service.Deps, error) {
	store, err := storage.NewMinIO(cfg.Storage)
	if err != nil {
		return service.Deps{}, err
	}
	// 수명 주기 규칙은 안전망이므로 설정하지 못해도 기동은 계속한다 (권한 없는 버킷 등).
	if err := store.EnsureTempLifecycle(ctx); err != nil {
		logger.Warn("temp bucket lifecycle not applied", slog.Any("error", err))
	}
	sender, err := newPushSender(ctx, cfg, logger)
	if err != nil {
		return service.Deps{}, err
	}
	dates := postgres.NewDateRepository(db)
	return service.Deps{
		Clock:      clock.System{},
		Tx:         postgres.NewTxManager(db),
		Users:      postgres.NewUserRepository(db),
		Identities: postgres.NewIdentityRepository(db),
		Ledger:     postgres.NewLedgerRepository(db),
		Refresh:    postgres.NewRefreshTokenRepository(db),
		Devices:    postgres.NewDeviceRepository(db),
		Couples:    postgres.NewCoupleRepository(db),
		Invites:    postgres.NewInviteRepository(db),
		Themes:     dates,
		Dates:      dates,
		Photos:     postgres.NewPhotoRepository(db),
		Storage:    store,
		Push:       sender,
		IDTokens:   idtoken.NewAppleGoogle(ctx, cfg.Auth.AppleClientIDs, cfg.Auth.GoogleClientIDs),
		Access:     token.NewJWTIssuer(cfg.Auth.JWTSecret),
		Logger:     logger,
	}, nil
}

// newPushSender 는 FCM 자격 증명이 있으면 FCM, 없으면 로그 출력 구현을 쓴다.
func newPushSender(ctx context.Context, cfg config.Config, logger *slog.Logger) (out.PushSender, error) {
	if cfg.Push.CredentialsFile == "" {
		logger.Info("FCM_CREDENTIALS_FILE is empty; push notifications are logged only")
		return push.NewLogSender(logger), nil
	}
	return push.NewFCMSender(ctx, cfg.Push.CredentialsFile)
}

// newAPI 는 /v1 아래 API 핸들러를 조립한다.
func newAPI(cfg config.Config, deps service.Deps, logger *slog.Logger) httpadapter.Handler {
	auth := service.NewAuthService(deps)
	photos := service.NewPhotoService(deps)
	handlers := []httpadapter.Handler{
		httpadapter.NewAuthHandler(auth, logger, cfg.App.IsLocal()),
		httpadapter.NewMeHandler(service.NewMeService(deps), auth, logger),
		httpadapter.NewCoupleHandler(service.NewCoupleService(deps), auth, logger),
		httpadapter.NewDateHandler(service.NewDateService(deps), photos, auth, logger),
		httpadapter.NewPhotoHandler(photos, auth, logger),
		httpadapter.NewDiaryHandler(service.NewDiaryService(deps), auth, logger),
	}
	// 관리자 키가 없으면 관리자 라우트를 등록하지 않는다.
	if cfg.Admin.APIKey != "" {
		handlers = append(handlers, httpadapter.NewAdminHandler(service.NewAdminService(deps), cfg.Admin.APIKey, logger))
	}
	return httpadapter.NewGroup("/v1", handlers...)
}
