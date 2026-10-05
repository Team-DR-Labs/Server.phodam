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

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required env")
	}
	for _, key := range []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME"} {
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
