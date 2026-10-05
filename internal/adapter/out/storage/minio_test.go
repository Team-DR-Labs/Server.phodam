package storage

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/config"
)

// 서명은 네트워크 없이 PublicEndpoint 호스트로 만들어져야 한다.
func TestMinIO_PresignUsesPublicEndpoint(t *testing.T) {
	m, err := NewMinIO(config.StorageConfig{
		Endpoint: "minio:9000", PublicEndpoint: "localhost:9000", AccessKey: "k", SecretKey: "s",
		Region: "us-east-1", TempBucket: "phodam-temp", PermanentBucket: "phodam-permanent",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	put, err := m.PresignPut(ctx, out.BucketTemp, "temp/d/u/p.jpg", "image/jpeg", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(put)
	if u.Host != "localhost:9000" || u.Path != "/phodam-temp/temp/d/u/p.jpg" {
		t.Fatalf("unexpected put url: %s", put)
	}
	if !strings.Contains(u.Query().Get("X-Amz-SignedHeaders"), "content-type") {
		t.Fatalf("content-type must be signed: %s", put)
	}

	get, err := m.PresignGet(ctx, out.BucketPermanent, "permanent/x.jpg", 15*time.Minute)
	if err != nil || !strings.HasPrefix(get, "http://localhost:9000/phodam-permanent/permanent/x.jpg?") {
		t.Fatalf("unexpected get url: %s, %v", get, err)
	}
}
