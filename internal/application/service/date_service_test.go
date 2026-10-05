package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

func TestDate_StartAssignsDistinctTopicsAndHidesPartnerTopic(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	svc := NewDateService(h.deps)

	v, err := svc.Start(ctx, a)
	mustOK(t, err)

	if v.Status != dating.StatusInProgress || !v.StartedByMe || !v.DeadlineAt.Equal(baseTime.Add(72*time.Hour)) {
		t.Fatalf("unexpected view: %+v", v)
	}
	if v.Me.Status != dating.ParticipantJoined || v.Me.Topic == nil {
		t.Fatalf("starter should see own topic: %+v", v.Me)
	}
	if v.Partner.Status != dating.ParticipantAssigned || v.Partner.Topic != nil {
		t.Fatalf("partner topic must be hidden: %+v", v.Partner)
	}
	d := h.st.dates[v.ID]
	if d.Participants[0].Topic.ID == d.Participants[1].Topic.ID {
		t.Fatal("topics must differ")
	}
	if got := h.push.typesTo("tok-bob"); len(got) != 1 || got[0] != push.TypeDateStarted {
		t.Fatalf("bob should get date_started, got %v", got)
	}

	bv, err := svc.Get(ctx, b, v.ID)
	mustOK(t, err)
	if bv.Me.Topic != nil || bv.Partner.Topic != nil || bv.StartedByMe {
		t.Fatalf("assigned partner sees no topics before join: %+v", bv)
	}
}

func TestDate_JoinRevealsOnlyOwnTopicAndIsIdempotent(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	svc := NewDateService(h.deps)
	v, err := svc.Start(ctx, a)
	mustOK(t, err)

	for i := 0; i < 2; i++ {
		jv, err := svc.Join(ctx, b, v.ID)
		mustOK(t, err)
		if jv.Me.Status != dating.ParticipantJoined || jv.Me.Topic == nil || jv.Partner.Topic != nil {
			t.Fatalf("join #%d: %+v", i, jv)
		}
	}
	av, err := svc.Get(ctx, a, v.ID)
	mustOK(t, err)
	if av.Partner.Topic != nil {
		t.Fatal("starter must not see partner topic before reveal")
	}
}

func TestDate_StartRejections(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	svc := NewDateService(h.deps)
	_, err := svc.Start(ctx, a)
	mustOK(t, err)

	_, err = svc.Start(ctx, b)
	wantCode(t, err, apperr.DateAlreadyInProgress)

	single := h.newUser("carol", 24)
	_, err = svc.Start(ctx, single)
	wantCode(t, err, apperr.CoupleRequired)
	_, err = svc.Current(ctx, single)
	wantCode(t, err, apperr.CoupleRequired)
}

func TestDate_StartAfterDeadlineExpiresPreviousImmediately(t *testing.T) {
	h := newHarness(t)
	a, _ := h.newCouple()
	svc := NewDateService(h.deps)
	first, err := svc.Start(ctx, a)
	mustOK(t, err)

	h.clock.Advance(dating.SubmitWindow)
	cur, err := svc.Current(ctx, a)
	mustOK(t, err)
	if cur != nil {
		t.Fatal("past-deadline date must not be current")
	}
	second, err := svc.Start(ctx, a)
	mustOK(t, err)
	if second.ID == first.ID || h.st.dates[first.ID].Status != dating.StatusExpired {
		t.Fatalf("previous date should be expired: %+v", h.st.dates[first.ID].Status)
	}
	old, err := svc.Get(ctx, a, first.ID)
	mustOK(t, err)
	if old.Status != dating.StatusExpired {
		t.Fatalf("status = %s", old.Status)
	}
}

func TestDate_ThemePrefersUnusedThenAvoidsLast(t *testing.T) {
	h := newHarness(t)
	a, _ := h.newCouple()
	svc := NewDateService(h.deps)
	seen := map[uuid.UUID]bool{}
	var last uuid.UUID
	for i := 0; i < 3; i++ {
		v, err := svc.Start(ctx, a)
		mustOK(t, err)
		if seen[v.Theme.ID] {
			t.Fatalf("theme %s repeated before all were used", v.Theme.Title)
		}
		seen[v.Theme.ID], last = true, v.Theme.ID
		h.clock.Advance(dating.SubmitWindow)
	}
	for i := 0; i < 5; i++ {
		v, err := svc.Start(ctx, a)
		mustOK(t, err)
		if v.Theme.ID == last {
			t.Fatal("must not repeat the previous theme")
		}
		last = v.Theme.ID
		h.clock.Advance(dating.SubmitWindow)
	}
}

func TestDate_GetOtherCouplesDateIsNotFound(t *testing.T) {
	h := newHarness(t)
	a, _ := h.newCouple()
	svc := NewDateService(h.deps)
	v, err := svc.Start(ctx, a)
	mustOK(t, err)

	c, d := h.newUser("carol", 24), h.newUser("dave", 24)
	_ = fakeCouples{h.st}.Create(ctx, coupleOf(c, d))
	_, err = svc.Get(ctx, c, v.ID)
	wantCode(t, err, apperr.NotFound)
	_, err = NewPhotoService(h.deps).ReserveShot(ctx, c, v.ID)
	wantCode(t, err, apperr.NotFound)
	_, err = svc.Get(ctx, c, uuid.New())
	wantCode(t, err, apperr.NotFound)
	if _, err := h.deps.Dates.Get(ctx, uuid.New()); err != out.ErrNotFound {
		t.Fatal("fake should return ErrNotFound")
	}
}
