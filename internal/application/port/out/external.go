package out

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// Bucket 은 논리 버킷이다. 실제 이름은 어댑터 설정이 정한다.
type Bucket string

const (
	BucketTemp      Bucket = "temp"
	BucketPermanent Bucket = "permanent"
)

// ObjectInfo 는 객체 메타데이터다.
type ObjectInfo struct {
	Size int64
}

// ObjectStorage 는 사진 객체 저장소다.
type ObjectStorage interface {
	// PresignPut 은 contentType 헤더를 서명에 포함한 PUT URL 을 만든다.
	PresignPut(ctx context.Context, bucket Bucket, key, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, bucket Bucket, key string, ttl time.Duration) (string, error)
	// Stat 은 객체 정보를 읽는다. 없으면 ErrObjectNotFound.
	Stat(ctx context.Context, bucket Bucket, key string) (ObjectInfo, error)
	Copy(ctx context.Context, src Bucket, srcKey string, dst Bucket, dstKey string) error
	// Remove 는 객체를 지운다. 이미 없으면 성공이다.
	Remove(ctx context.Context, bucket Bucket, key string) error
}

// PushSender 는 푸시 발송기다. 무효(UNREGISTERED) 토큰을 돌려준다.
type PushSender interface {
	Send(ctx context.Context, tokens []string, msg push.Message) (invalid []string, err error)
}

// IDTokenClaims 는 검증된 외부 ID 토큰의 필요한 클레임이다.
type IDTokenClaims struct {
	Subject string
	Name    string
}

// IDTokenVerifier 는 Apple/Google ID 토큰을 JWKS 로 검증한다.
type IDTokenVerifier interface {
	Verify(ctx context.Context, provider user.Provider, token string) (IDTokenClaims, error)
}

// AccessTokenIssuer 는 자체 액세스 토큰(JWT)을 발급·검증한다.
type AccessTokenIssuer interface {
	Issue(userID uuid.UUID, now time.Time) (token string, expiresAt time.Time, err error)
	// Parse 는 토큰을 검증하고 사용자 ID 를 반환한다.
	Parse(token string, now time.Time) (uuid.UUID, error)
}
