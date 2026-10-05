package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

func TestPhoto_ReserveShotDeductsFilmAndRecordsLedger(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)

	res, err := NewPhotoService(h.deps).ReserveShot(ctx, a, dateID)
	mustOK(t, err)

	if res.FilmBalance != 23 || h.st.users[a].FilmBalance != 23 || h.st.ledgerSum(a) != 23 {
		t.Fatalf("balance = %d, stored = %d, ledger = %d", res.FilmBalance, h.st.users[a].FilmBalance, h.st.ledgerSum(a))
	}
	last := h.st.ledger[len(h.st.ledger)-1]
	if last.Reason != user.LedgerShot || last.Delta != -1 || last.RefID == nil || *last.RefID != res.Photo.ID {
		t.Fatalf("unexpected ledger entry: %+v", last)
	}
	wantKey := "temp/" + dateID.String() + "/" + a.String() + "/" + res.Photo.ID.String() + ".jpg"
	if res.Photo.Status != photo.StatusReserved || res.Photo.TempKey != wantKey {
		t.Fatalf("unexpected photo: %+v", res.Photo)
	}
	if res.Upload.Method != "PUT" || res.Upload.Headers["Content-Type"] != "image/jpeg" || !res.Upload.ExpiresAt.Equal(baseTime.Add(photo.URLTTL)) {
		t.Fatalf("unexpected upload target: %+v", res.Upload)
	}
}

func TestPhoto_ReserveShotRejectsZeroFilmWithoutLedger(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	h.st.users[a] = withBalance(h.st.users[a], 0)
	h.st.ledger = append(h.st.ledger, user.LedgerEntry{UserID: a, Delta: -user.SignupFilm})
	before := len(h.st.ledger)

	_, err := NewPhotoService(h.deps).ReserveShot(ctx, a, dateID)

	wantCode(t, err, apperr.FilmExhausted)
	if len(h.st.ledger) != before || len(h.st.photos) != 0 || h.st.users[a].FilmBalance != 0 {
		t.Fatal("rejected shot must not change ledger, photos or balance")
	}
}

func TestPhoto_ReserveShotConditions(t *testing.T) {
	tests := []struct {
		name string
		prep func(t *testing.T, h *harness, a, b, dateID uuid.UUID) uuid.UUID
		want apperr.Code
	}{
		{"before join (assigned)", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) uuid.UUID { return b }, apperr.DateNotJoined},
		{"after deadline", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) uuid.UUID {
			h.clock.Advance(dating.SubmitWindow)
			return a
		}, apperr.DateNotActive},
		{"after submit", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) uuid.UUID {
			_, _ = NewDateService(h.deps).Join(ctx, b, dateID)
			h.submit(t, a, dateID, h.shoot(t, a, dateID), "")
			return a
		}, apperr.AlreadySubmitted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			a, b := h.newCouple()
			v, err := NewDateService(h.deps).Start(ctx, a)
			mustOK(t, err)
			who := tt.prep(t, h, a, b, v.ID)
			before := h.st.users[who].FilmBalance

			_, err = NewPhotoService(h.deps).ReserveShot(ctx, who, v.ID)

			wantCode(t, err, tt.want)
			if h.st.users[who].FilmBalance != before {
				t.Fatal("film must not be deducted on rejection")
			}
		})
	}
}

func TestPhoto_CompleteUpload(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	svc := NewPhotoService(h.deps)
	res, err := svc.ReserveShot(ctx, a, dateID)
	mustOK(t, err)

	_, err = svc.CompleteUpload(ctx, a, res.Photo.ID)
	wantCode(t, err, apperr.PhotoNotUploaded)

	_, err = svc.CompleteUpload(ctx, b, res.Photo.ID)
	wantCode(t, err, apperr.NotFound)

	h.storage.put(out.BucketTemp, res.Photo.TempKey, photo.MaxSizeBytes+1)
	_, err = svc.CompleteUpload(ctx, a, res.Photo.ID)
	wantCode(t, err, apperr.PhotoInvalid)
	if h.storage.has(out.BucketTemp, res.Photo.TempKey) {
		t.Fatal("oversized object must be removed")
	}

	h.storage.put(out.BucketTemp, res.Photo.TempKey, 2048)
	for i := 0; i < 2; i++ {
		p, err := svc.CompleteUpload(ctx, a, res.Photo.ID)
		mustOK(t, err)
		if p.Status != photo.StatusUploaded || *p.SizeBytes != 2048 {
			t.Fatalf("complete #%d: %+v", i, p)
		}
	}
	_, err = svc.ReissueUploadURL(ctx, a, res.Photo.ID)
	wantCode(t, err, apperr.NotFound)
}

func TestPhoto_ReissueUploadURLAndMyPhotos(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	svc := NewPhotoService(h.deps)
	res, err := svc.ReserveShot(ctx, a, dateID)
	mustOK(t, err)

	target, err := svc.ReissueUploadURL(ctx, a, res.Photo.ID)
	mustOK(t, err)
	if target.URL == "" || target.Method != "PUT" {
		t.Fatalf("unexpected target: %+v", target)
	}

	uploaded := h.shoot(t, a, dateID)
	items, err := svc.ListMine(ctx, a, dateID)
	mustOK(t, err)
	if len(items) != 1 || items[0].Photo.ID != uploaded {
		t.Fatalf("only uploaded photos are listed: %+v", items)
	}
	partnerItems, err := svc.ListMine(ctx, b, dateID)
	mustOK(t, err)
	if len(partnerItems) != 0 {
		t.Fatal("partner must not see my photos")
	}
}

func withBalance(u user.User, n int) user.User {
	u.FilmBalance = n
	return u
}
