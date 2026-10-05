package service

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

func TestSubmit_ArchivesRepresentativeAndCleansUp(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	rep := h.shoot(t, a, dateID)
	extra := h.shoot(t, a, dateID)
	pending, err := NewPhotoService(h.deps).ReserveShot(ctx, a, dateID)
	mustOK(t, err)

	v, err := NewDateService(h.deps).Submit(ctx, a, dateID, rep, ptr("  오늘 좋았다  "))
	mustOK(t, err)

	if v.Me.Status != dating.ParticipantSubmitted || !v.Me.ReceiveDeadlineAt.Equal(baseTime.Add(dating.ReceiveWindow)) {
		t.Fatalf("unexpected me: %+v", v.Me)
	}
	if v.Status != dating.StatusInProgress || v.Partner.Topic != nil {
		t.Fatalf("partner topic must stay hidden until both submit: %+v", v)
	}
	p := h.st.photos[rep]
	permKey := photo.PermanentKey(dateID, a, rep)
	if p.Status != photo.StatusArchived || !p.IsRepresentative || p.PermanentKey == nil || *p.PermanentKey != permKey || p.TempPurgedAt == nil {
		t.Fatalf("unexpected representative: %+v", p)
	}
	if !h.storage.has(out.BucketPermanent, permKey) || h.storage.has(out.BucketTemp, p.TempKey) {
		t.Fatal("representative must be copied to permanent and removed from temp")
	}
	if h.st.photos[extra].Status != photo.StatusUploaded || !h.storage.has(out.BucketTemp, h.st.photos[extra].TempKey) {
		t.Fatal("other uploaded photos stay receivable")
	}
	if h.st.photos[pending.Photo.ID].Status != photo.StatusDeleted {
		t.Fatal("reserved shots are deleted on submit")
	}
	me, _ := h.st.dates[dateID].Participant(a)
	if me.Caption == nil || *me.Caption != "오늘 좋았다" || *me.RepresentativePhotoID != rep {
		t.Fatalf("unexpected participant: %+v", me)
	}
	if got := h.push.typesTo("tok-bob"); got[len(got)-1] != push.TypePartnerSubmitted {
		t.Fatalf("bob should get partner_submitted, got %v", got)
	}
}

func TestSubmit_BothSubmittedReveals(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	h.submit(t, a, dateID, h.shoot(t, a, dateID), "")
	v, err := NewDateService(h.deps).Submit(ctx, b, dateID, h.shoot(t, b, dateID), nil)
	mustOK(t, err)

	if v.Status != dating.StatusRevealed || v.RevealedAt == nil || v.Partner.Topic == nil {
		t.Fatalf("date should be revealed with partner topic: %+v", v)
	}
	for _, tok := range []string{"tok-alice", "tok-bob"} {
		got := h.push.typesTo(tok)
		if got[len(got)-1] != push.TypeDateRevealed {
			t.Fatalf("%s should get date_revealed, got %v", tok, got)
		}
	}
	bp, _ := h.st.dates[dateID].Participant(b)
	if bp.Caption != nil {
		t.Fatal("nil caption stays nil")
	}
}

func TestSubmit_Rejections(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, h *harness, a, b, dateID uuid.UUID) error
		want apperr.Code
	}{
		{"twice", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) error {
			p := h.shoot(t, a, dateID)
			h.submit(t, a, dateID, p, "")
			_, err := NewDateService(h.deps).Submit(ctx, a, dateID, p, nil)
			return err
		}, apperr.AlreadySubmitted},
		{"after deadline", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) error {
			p := h.shoot(t, a, dateID)
			h.clock.Advance(dating.SubmitWindow)
			_, err := NewDateService(h.deps).Submit(ctx, a, dateID, p, nil)
			return err
		}, apperr.DateNotActive},
		{"not uploaded", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) error {
			res, _ := NewPhotoService(h.deps).ReserveShot(ctx, a, dateID)
			_, err := NewDateService(h.deps).Submit(ctx, a, dateID, res.Photo.ID, nil)
			return err
		}, apperr.PhotoNotUploaded},
		{"partner's photo", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) error {
			_, err := NewDateService(h.deps).Submit(ctx, a, dateID, h.shoot(t, b, dateID), nil)
			return err
		}, apperr.NotFound},
		{"overwritten after complete", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) error {
			p := h.shoot(t, a, dateID)
			h.storage.put(out.BucketTemp, h.st.photos[p].TempKey, photo.MaxSizeBytes+1)
			_, err := NewDateService(h.deps).Submit(ctx, a, dateID, p, nil)
			return err
		}, apperr.PhotoInvalid},
		{"caption too long", func(t *testing.T, h *harness, a, b, dateID uuid.UUID) error {
			_, err := NewDateService(h.deps).Submit(ctx, a, dateID, h.shoot(t, a, dateID), ptr(strings.Repeat("가", 201)))
			return err
		}, apperr.ValidationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			a, b := h.newCouple()
			dateID := h.startJoined(t, a, b)
			wantCode(t, tt.run(t, h, a, b, dateID), tt.want)
		})
	}
}

func TestSubmit_TempRemoveFailureStillSucceeds(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	rep := h.shoot(t, a, dateID)
	h.storage.failRemove = true

	_, err := NewDateService(h.deps).Submit(ctx, a, dateID, rep, nil)

	mustOK(t, err)
	if p := h.st.photos[rep]; p.Status != photo.StatusArchived || p.TempPurgedAt != nil {
		t.Fatalf("archived with temp left for the worker: %+v", p)
	}
}

func TestSubmit_CaptionAt200RunesAccepted(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	_, err := NewDateService(h.deps).Submit(ctx, a, dateID, h.shoot(t, a, dateID), ptr(strings.Repeat("가", 200)))
	mustOK(t, err)
}
