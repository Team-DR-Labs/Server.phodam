package config

import (
	"strings"
	"testing"
	"time"
)

func setDBEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_USER", "phodam")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "phodam")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("STORAGE_ENDPOINT", "minio:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "key")
	t.Setenv("STORAGE_SECRET_KEY", "secret")
}

func TestLoad_Defaults(t *testing.T) {
	setDBEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.App.Port != 8080 || cfg.DB.Port != 5432 || cfg.DB.ConnMaxLifetime != 30*time.Minute {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("STORAGE_ENDPOINT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required env")
	}
	for _, key := range []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME", "JWT_SECRET", "STORAGE_ENDPOINT"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error should mention %s: %v", key, err)
		}
	}
}

func TestLoad_InvalidNumber(t *testing.T) {
	setDBEnv(t)
	t.Setenv("APP_PORT", "abc")

	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "APP_PORT") {
		t.Fatalf("expected APP_PORT error, got %v", err)
	}
}

func TestLoad_AppDefaults(t *testing.T) {
	setDBEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	st := cfg.Storage
	if st.PublicEndpoint != "minio:9000" || st.TempBucket != "phodam-temp" || st.PermanentBucket != "phodam-permanent" || st.Region != "us-east-1" {
		t.Fatalf("unexpected storage defaults: %+v", st)
	}
	if cfg.App.Env != "production" || cfg.App.IsLocal() {
		t.Fatalf("APP_ENV must default to production, got %q", cfg.App.Env)
	}
	if cfg.Worker.Interval != time.Minute || cfg.Admin.APIKey != "" || cfg.Push.CredentialsFile != "" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoad_ClientIDLists(t *testing.T) {
	setDBEnv(t)
	t.Setenv("APPLE_CLIENT_IDS", " com.phodam.app , com.phodam.dev,")
	t.Setenv("STORAGE_PUBLIC_ENDPOINT", "localhost:9000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := cfg.Auth.AppleClientIDs; len(got) != 2 || got[0] != "com.phodam.app" || got[1] != "com.phodam.dev" {
		t.Fatalf("apple client ids = %v", got)
	}
	if cfg.Auth.GoogleClientIDs != nil || cfg.Storage.PublicEndpoint != "localhost:9000" {
		t.Fatalf("unexpected config: %+v", cfg.Auth)
	}
}

func TestLoad_InvalidValues(t *testing.T) {
	tests := []struct{ key, value string }{
		{"JWT_SECRET", "short"},
		{"STORAGE_USE_SSL", "maybe"},
		{"WORKER_INTERVAL", "0s"},
		{"ADMIN_API_KEY", "short-key"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			setDBEnv(t)
			t.Setenv(tt.key, tt.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("expected %s error, got %v", tt.key, err)
			}
		})
	}
}

func TestLoad_ShortAdminKeyAllowedLocally(t *testing.T) {
	setDBEnv(t)
	t.Setenv("APP_ENV", "local")
	t.Setenv("ADMIN_API_KEY", "local-admin-key")
	if _, err := Load(); err != nil {
		t.Fatalf("local may use a short admin key: %v", err)
	}
}
