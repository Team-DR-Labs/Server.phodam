package service

import (
	"context"
	"fmt"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
)

// presignPhoto 는 사진 위치(archived 면 permanent, 그 외 temp)의 GET URL 을 만든다.
func presignPhoto(ctx context.Context, d Deps, p photo.Photo) (diary.URL, error) {
	bucket, key := out.BucketTemp, p.TempKey
	if p.Status == photo.StatusArchived && p.PermanentKey != nil {
		bucket, key = out.BucketPermanent, *p.PermanentKey
	}
	return presignKey(ctx, d, bucket, key)
}

func presignKey(ctx context.Context, d Deps, bucket out.Bucket, key string) (diary.URL, error) {
	expires := d.now().Add(photo.URLTTL)
	u, err := d.Storage.PresignGet(ctx, bucket, key, photo.URLTTL)
	if err != nil {
		return diary.URL{}, fmt.Errorf("presign get: %w", err)
	}
	return diary.URL{URL: u, ExpiresAt: expires}, nil
}
