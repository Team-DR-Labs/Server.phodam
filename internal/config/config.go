// Package config 는 환경 변수에서 애플리케이션 설정을 읽고 검증한다.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 는 애플리케이션 전체 설정이다.
type Config struct {
	App AppConfig
	DB  DBConfig
}

// AppConfig 는 HTTP 서버 설정이다.
type AppConfig struct {
	Env             string
	Port            int
	GinMode         string
	ShutdownTimeout time.Duration
}

// DBConfig 는 PostgreSQL 연결 설정이다.
type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// DSN 은 gorm postgres 드라이버용 연결 문자열을 반환한다.
// 비밀번호가 포함되므로 로그에 출력하지 않는다.
func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=UTC",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// Load 는 환경 변수에서 설정을 읽는다. 필수 값이 없거나 형식이 틀리면 에러를 반환한다.
func Load() (Config, error) {
	r := &reader{}

	cfg := Config{
		App: AppConfig{
			Env:             r.str("APP_ENV", "local"),
			Port:            r.int("APP_PORT", 8080),
			GinMode:         r.str("GIN_MODE", "release"),
			ShutdownTimeout: r.duration("APP_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		DB: DBConfig{
			Host:            r.required("DB_HOST"),
			Port:            r.int("DB_PORT", 5432),
			User:            r.required("DB_USER"),
			Password:        r.required("DB_PASSWORD"),
			Name:            r.required("DB_NAME"),
			SSLMode:         r.str("DB_SSLMODE", "disable"),
			MaxOpenConns:    r.int("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    r.int("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: r.duration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		},
	}

	if err := errors.Join(r.errs...); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// reader 는 환경 변수를 읽으며 발생한 에러를 모아 둔다.
type reader struct {
	errs []error
}

func (r *reader) str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func (r *reader) required(key string) string {
	v := os.Getenv(key)
	if v == "" {
		r.errs = append(r.errs, fmt.Errorf("%s is required", key))
	}
	return v
}

func (r *reader) int(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s must be an integer: %w", key, err))
		return def
	}
	return n
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s must be a duration (e.g. 30s): %w", key, err))
		return def
	}
	return d
}
