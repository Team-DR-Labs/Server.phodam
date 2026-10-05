// Package config 는 환경 변수에서 애플리케이션 설정을 읽고 검증한다.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 는 애플리케이션 전체 설정이다.
type Config struct {
	App     AppConfig
	DB      DBConfig
	Auth    AuthConfig
	Admin   AdminConfig
	Storage StorageConfig
	Push    PushConfig
	Worker  WorkerConfig
}

// AuthConfig 는 토큰 발급과 외부 ID 토큰 검증 설정이다.
type AuthConfig struct {
	// JWTSecret 은 액세스 토큰 HS256 서명 키다 (32바이트 이상).
	JWTSecret string
	// AppleClientIDs 는 Apple ID 토큰 aud 로 허용할 번들 ID 목록이다.
	AppleClientIDs []string
	// GoogleClientIDs 는 Google ID 토큰 aud 로 허용할 클라이언트 ID 목록이다.
	GoogleClientIDs []string
}

// AdminConfig 는 관리자 API 설정이다. APIKey 가 비어 있으면 관리자 라우트를 등록하지 않는다.
type AdminConfig struct {
	APIKey string
}

// StorageConfig 는 S3 호환(MinIO) 오브젝트 스토리지 설정이다.
type StorageConfig struct {
	// Endpoint 는 서버가 직접 접근하는 주소다 (예: minio:9000).
	Endpoint string
	// PublicEndpoint 는 presigned URL 서명에 쓰는 앱 접근 주소다 (예: localhost:9000).
	PublicEndpoint  string
	AccessKey       string
	SecretKey       string
	UseSSL          bool
	PublicUseSSL    bool
	Region          string
	TempBucket      string
	PermanentBucket string
}

// PushConfig 는 FCM 설정이다. CredentialsFile 이 비어 있으면 로그 출력 구현을 쓴다.
type PushConfig struct {
	CredentialsFile string
}

// WorkerConfig 는 주기 작업 설정이다.
type WorkerConfig struct {
	Interval time.Duration
}

// IsLocal 은 로컬 개발 환경인지 반환한다 (dev 로그인 라우트 등록 조건).
func (c AppConfig) IsLocal() bool { return c.Env == "local" }

// minJWTSecretLen 은 HS256 키 최소 길이다.
const minJWTSecretLen = 32

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
		Auth: AuthConfig{
			JWTSecret:       r.required("JWT_SECRET"),
			AppleClientIDs:  r.list("APPLE_CLIENT_IDS"),
			GoogleClientIDs: r.list("GOOGLE_CLIENT_IDS"),
		},
		Admin: AdminConfig{APIKey: r.str("ADMIN_API_KEY", "")},
		Push:  PushConfig{CredentialsFile: r.str("FCM_CREDENTIALS_FILE", "")},
		Worker: WorkerConfig{
			Interval: r.duration("WORKER_INTERVAL", time.Minute),
		},
	}
	cfg.Storage = loadStorage(r)
	r.validate(cfg)

	if err := errors.Join(r.errs...); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func loadStorage(r *reader) StorageConfig {
	endpoint := r.required("STORAGE_ENDPOINT")
	useSSL := r.bool("STORAGE_USE_SSL", false)
	return StorageConfig{
		Endpoint:        endpoint,
		PublicEndpoint:  r.str("STORAGE_PUBLIC_ENDPOINT", endpoint),
		AccessKey:       r.required("STORAGE_ACCESS_KEY"),
		SecretKey:       r.required("STORAGE_SECRET_KEY"),
		UseSSL:          useSSL,
		PublicUseSSL:    r.bool("STORAGE_PUBLIC_USE_SSL", useSSL),
		Region:          r.str("STORAGE_REGION", "us-east-1"),
		TempBucket:      r.str("STORAGE_TEMP_BUCKET", "phodam-temp"),
		PermanentBucket: r.str("STORAGE_PERMANENT_BUCKET", "phodam-permanent"),
	}
}

func (r *reader) validate(cfg Config) {
	if s := cfg.Auth.JWTSecret; s != "" && len(s) < minJWTSecretLen {
		r.errs = append(r.errs, fmt.Errorf("JWT_SECRET must be at least %d bytes", minJWTSecretLen))
	}
	if cfg.Worker.Interval <= 0 {
		r.errs = append(r.errs, errors.New("WORKER_INTERVAL must be positive"))
	}
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

// list 는 콤마로 구분된 값을 공백을 지워 읽는다. 비어 있으면 nil 이다.
func (r *reader) list(key string) []string {
	var res []string
	for _, v := range strings.Split(os.Getenv(key), ",") {
		if v = strings.TrimSpace(v); v != "" {
			res = append(res, v)
		}
	}
	return res
}

func (r *reader) bool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.errs = append(r.errs, fmt.Errorf("%s must be a boolean: %w", key, err))
		return def
	}
	return b
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
