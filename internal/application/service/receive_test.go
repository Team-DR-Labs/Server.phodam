package service

import (
	"testing"
	"time"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
)

func TestReceive_OnlyAfterSubmitAndWithinSevenDays(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	ps := NewPhotoService(h.deps)
	rep, extra := h.shoot(t, a, dateID), h.shoot(t, a, dateID)

	_, err := ps.Receivable(ctx, a, dateID)
	wantCode(t, err, apperr.ReceiveNotAvailable)
	_, err = ps.AckReceived(ctx, a, extra)
	wantCode(t, err, apperr.ReceiveNotAvailable)

	h.submit(t, a, dateID, rep, "")
	got, err := ps.Receivable(ctx, a, dateID)
	mustOK(t, err)
	if len(got.Items) != 2 || !got.ReceiveDeadlineAt.Equal(baseTime.Add(dating.ReceiveWindow)) {
		t.Fatalf("receivable = %+v", got)
	}
	_, err = ps.Receivable(ctx, b, dateID)
	wantCode(t, err, apperr.ReceiveNotAvailable)

	h.clock.Advance(dating.ReceiveWindow)
	_, err = ps.Receivable(ctx, a, dateID)
	wantCode(t, err, apperr.ReceiveNotAvailable)
	_, err = ps.AckReceived(ctx, a, extra)
	wantCode(t, err, apperr.ReceiveNotAvailable)
}

func TestReceive_AckDeletesNonRepresentativeIdempotently(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	ps := NewPhotoService(h.deps)
	rep, extra := h.shoot(t, a, dateID), h.shoot(t, a, dateID)
	h.submit(t, a, dateID, rep, "")
	tempKey := h.st.photos[extra].TempKey

	for i := 0; i < 2; i++ {
		p, err := ps.AckReceived(ctx, a, extra)
		mustOK(t, err)
		if p.Status != photo.StatusReceived || p.ReceivedAt == nil || !p.ReceivedAt.Equal(baseTime) {
			t.Fatalf("ack #%d: %+v", i, p)
		}
		h.clock.Advance(time.Minute)
	}
	if h.storage.has(out.BucketTemp, tempKey) {
		t.Fatal("temp object must be deleted after ack")
	}
	got, err := ps.Receivable(ctx, a, dateID)
	mustOK(t, err)
	if len(got.Items) != 1 || got.Items[0].Photo.ID != rep {
		t.Fatalf("only representative remains receivable: %+v", got.Items)
	}
	_, err = ps.AckReceived(ctx, b, extra)
	wantCode(t, err, apperr.NotFound)
}

func TestReceive_AckRepresentativeKeepsPermanentCopy(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	rep := h.shoot(t, a, dateID)
	h.submit(t, a, dateID, rep, "")

	p, err := NewPhotoService(h.deps).AckReceived(ctx, a, rep)
	mustOK(t, err)

	if p.Status != photo.StatusArchived || p.ReceivedAt == nil {
		t.Fatalf("representative stays archived with received_at: %+v", p)
	}
	if !h.storage.has(out.BucketPermanent, photo.PermanentKey(dateID, a, rep)) {
		t.Fatal("permanent copy must not be deleted")
	}
	got, err := NewPhotoService(h.deps).Receivable(ctx, a, dateID)
	mustOK(t, err)
	if len(got.Items) != 1 || got.Items[0].Photo.ID != rep {
		t.Fatal("representative stays in receivable regardless of received_at")
	}
	detail, err := NewDiaryService(h.deps).Get(ctx, a, dateID)
	mustOK(t, err)
	if len(detail.Entries) != 1 || detail.Entries[0].Photo.URL == "" {
		t.Fatalf("diary keeps showing the representative: %+v", detail)
	}
}

func TestReceive_AckFailureKeepsPhoto(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	rep, extra := h.shoot(t, a, dateID), h.shoot(t, a, dateID)
	h.submit(t, a, dateID, rep, "")
	h.storage.failRemove = true

	_, err := NewPhotoService(h.deps).AckReceived(ctx, a, extra)

	if err == nil || h.st.photos[extra].Status != photo.StatusUploaded {
		t.Fatalf("failed delete keeps photo uploaded: err=%v status=%s", err, h.st.photos[extra].Status)
	}
}
