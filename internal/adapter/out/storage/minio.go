// Package storage 는 S3 호환(MinIO) 오브젝트 스토리지 아웃바운드 어댑터다.
package storage

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/config"
)

// MinIO 는 out.ObjectStorage 구현체다.
//
// 서명에는 Host 가 포함되므로 presigned URL 은 앱이 접근할 PublicEndpoint 로 만든 별도 클라이언트로 서명한다.
// 서명 클라이언트는 Region 을 명시해 버킷 위치 조회(네트워크) 없이 서명만 한다.
type MinIO struct {
	internal *minio.Client
	signer   *minio.Client
	buckets  map[out.Bucket]string
}

var _ out.ObjectStorage = (*MinIO)(nil)

// NewMinIO 는 내부 접근용과 presign 서명용 클라이언트를 만든다.
func NewMinIO(cfg config.StorageConfig) (*MinIO, error) {
	internal, err := newClient(cfg.Endpoint, cfg.UseSSL, cfg)
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}
	signer, err := newClient(cfg.PublicEndpoint, cfg.PublicUseSSL, cfg)
	if err != nil {
		return nil, fmt.Errorf("storage signer: %w", err)
	}
	return &MinIO{
		internal: internal,
		signer:   signer,
		buckets: map[out.Bucket]string{
			out.BucketTemp:      cfg.TempBucket,
			out.BucketPermanent: cfg.PermanentBucket,
		},
	}, nil
}

func newClient(endpoint string, secure bool, cfg config.StorageConfig) (*minio.Client, error) {
	return minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
}

// PresignPut 은 Content-Type 을 서명에 포함한 PUT URL 을 만든다. 앱은 같은 Content-Type 으로 보내야 한다.
func (m *MinIO) PresignPut(ctx context.Context, b out.Bucket, key, contentType string, ttl time.Duration) (string, error) {
	h := http.Header{"Content-Type": []string{contentType}}
	u, err := m.signer.PresignHeader(ctx, http.MethodPut, m.buckets[b], key, ttl, nil, h)
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return u.String(), nil
}

// PresignGet 은 GET URL 을 만든다.
func (m *MinIO) PresignGet(ctx context.Context, b out.Bucket, key string, ttl time.Duration) (string, error) {
	u, err := m.signer.PresignedGetObject(ctx, m.buckets[b], key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}
	return u.String(), nil
}

// Stat 은 객체 크기를 읽는다.
func (m *MinIO) Stat(ctx context.Context, b out.Bucket, key string) (out.ObjectInfo, error) {
	info, err := m.internal.StatObject(ctx, m.buckets[b], key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFound(err) {
			return out.ObjectInfo{}, out.ErrObjectNotFound
		}
		return out.ObjectInfo{}, fmt.Errorf("stat object: %w", err)
	}
	return out.ObjectInfo{Size: info.Size}, nil
}

// Copy 는 서버 측 복사를 한다.
func (m *MinIO) Copy(ctx context.Context, src out.Bucket, srcKey string, dst out.Bucket, dstKey string) error {
	_, err := m.internal.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: m.buckets[dst], Object: dstKey},
		minio.CopySrcOptions{Bucket: m.buckets[src], Object: srcKey},
	)
	if err != nil {
		return fmt.Errorf("copy object: %w", err)
	}
	return nil
}

// Remove 는 객체를 지운다. 이미 없으면 성공이다.
func (m *MinIO) Remove(ctx context.Context, b out.Bucket, key string) error {
	err := m.internal.RemoveObject(ctx, m.buckets[b], key, minio.RemoveObjectOptions{})
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("remove object: %w", err)
	}
	return nil
}

// TempSafetyNetDays 는 temp 객체를 수명 주기 규칙으로 지우는 기한이다.
// 정책상 temp 객체는 업로드 후 길어야 제출 마감(3일) + 수령 기한(7일) 동안만 필요하다.
// 상태가 deleted/received 가 된 뒤 남은 presigned PUT 으로 다시 올라온 고아 객체를 지우는 안전망이다.
const TempSafetyNetDays = 11

// EnsureTempLifecycle 은 temp 버킷에 만료 규칙을 설정한다 (전체 설정을 덮어쓰므로 멱등).
func (m *MinIO) EnsureTempLifecycle(ctx context.Context) error {
	cfg := lifecycle.NewConfiguration()
	cfg.Rules = []lifecycle.Rule{{
		ID:         "phodam-temp-safety-net",
		Status:     "Enabled",
		RuleFilter: lifecycle.Filter{Prefix: "temp/"},
		Expiration: lifecycle.Expiration{Days: lifecycle.ExpirationDays(TempSafetyNetDays)},
	}}
	if err := m.internal.SetBucketLifecycle(ctx, m.buckets[out.BucketTemp], cfg); err != nil {
		return fmt.Errorf("set temp bucket lifecycle: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	code := minio.ToErrorResponse(err).Code
	return code == minio.NoSuchKey || code == "NoSuchKey" || code == "NotFound"
}
