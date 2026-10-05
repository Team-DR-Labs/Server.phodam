package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
)

// startJoined 는 a 가 데이트를 시작하고 b 가 참여한 상태를 만든다.
func (h *harness) startJoined(t *testing.T, a, b uuid.UUID) uuid.UUID {
	t.Helper()
	ds := NewDateService(h.deps)
	v, err := ds.Start(ctx, a)
	mustOK(t, err)
	_, err = ds.Join(ctx, b, v.ID)
	mustOK(t, err)
	return v.ID
}

// shoot 은 샷을 예약하고 temp 에 업로드한 뒤 complete 까지 마친 사진 ID 를 반환한다.
func (h *harness) shoot(t *testing.T, userID, dateID uuid.UUID) uuid.UUID {
	t.Helper()
	ps := NewPhotoService(h.deps)
	res, err := ps.ReserveShot(ctx, userID, dateID)
	mustOK(t, err)
	h.storage.put(out.BucketTemp, res.Photo.TempKey, 1024)
	p, err := ps.CompleteUpload(ctx, userID, res.Photo.ID)
	mustOK(t, err)
	if p.Status != photo.StatusUploaded {
		t.Fatalf("status = %s, want uploaded", p.Status)
	}
	return p.ID
}

func (h *harness) submit(t *testing.T, userID, dateID, photoID uuid.UUID, caption string) {
	t.Helper()
	_, err := NewDateService(h.deps).Submit(ctx, userID, dateID, photoID, &caption)
	mustOK(t, err)
}

func coupleOf(a, b uuid.UUID) couple.Couple {
	return couple.Couple{ID: uuid.New(), UserAID: a, UserBID: b, Status: couple.StatusActive, ConnectedAt: baseTime}
}
