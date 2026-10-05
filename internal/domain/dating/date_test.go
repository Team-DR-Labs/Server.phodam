package dating

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

var now = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)

func newDate() (Date, uuid.UUID, uuid.UUID) {
	a, b := uuid.New(), uuid.New()
	return Start(StartParams{
		ID: uuid.New(), CoupleID: uuid.New(), Theme: Theme{ID: uuid.New(), Title: "온기"},
		Starter: a, Partner: b, StarterTopic: Topic{ID: uuid.New(), Title: "a"}, PartnerTopic: Topic{ID: uuid.New(), Title: "b"}, Now: now,
	}), a, b
}

func TestCheckCanCapture(t *testing.T) {
	d, a, b := newDate()
	submitted, _, _ := d.Submit(a, uuid.New(), nil, now)
	tests := []struct {
		name string
		d    Date
		who  uuid.UUID
		at   time.Time
		want apperr.Code
	}{
		{"starter ok", d, a, now, ""},
		{"partner not joined", d, b, now, apperr.DateNotJoined},
		{"deadline reached", d, a, now.Add(SubmitWindow), apperr.DateNotActive},
		{"already submitted", submitted, a, now, apperr.AlreadySubmitted},
		{"stranger", d, uuid.New(), now, apperr.NotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.d.CheckCanCapture(tt.who, tt.at)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !apperr.Is(err, tt.want) {
				t.Fatalf("err = %v, want %s", err, tt.want)
			}
		})
	}
}

func TestSubmitDoesNotMutateOriginal(t *testing.T) {
	d, a, _ := newDate()
	next, revealed, err := d.Submit(a, uuid.New(), nil, now)
	if err != nil || revealed {
		t.Fatalf("submit: revealed=%v err=%v", revealed, err)
	}
	if p, _ := d.Participant(a); p.HasSubmitted() {
		t.Fatal("original date must not change")
	}
	if p, _ := next.Participant(a); !p.HasSubmitted() || !p.ReceiveDeadlineAt.Equal(now.Add(ReceiveWindow)) {
		t.Fatalf("unexpected participant: %+v", p)
	}
}

func TestViewForHidesPartnerTopicUntilRevealed(t *testing.T) {
	d, a, b := newDate()
	d, _, _ = d.Join(b, now)
	d, _, _ = d.Submit(a, uuid.New(), nil, now)
	if v := d.ViewFor(b, user.Profile{ID: a}, 0, now); v.Partner.Topic != nil || v.Me.Topic == nil {
		t.Fatalf("before reveal: %+v", v)
	}
	d, revealed, _ := d.Submit(b, uuid.New(), nil, now)
	if !revealed {
		t.Fatal("second submit reveals")
	}
	if v := d.ViewFor(b, user.Profile{ID: a}, 0, now); v.Partner.Topic == nil || v.Partner.Topic.Title != "a" {
		t.Fatalf("after reveal: %+v", v)
	}
}

func TestThemeCandidates(t *testing.T) {
	t1, t2 := Theme{ID: uuid.New()}, Theme{ID: uuid.New()}
	all := []Theme{t1, t2}
	if got := ThemeCandidates(all, map[uuid.UUID]bool{t1.ID: true}, &t1.ID); len(got) != 1 || got[0] != t2 {
		t.Fatalf("unused first: %v", got)
	}
	if got := ThemeCandidates(all, map[uuid.UUID]bool{t1.ID: true, t2.ID: true}, &t2.ID); len(got) != 1 || got[0] != t1 {
		t.Fatalf("all used -> exclude last: %v", got)
	}
	if got := ThemeCandidates([]Theme{t1}, map[uuid.UUID]bool{t1.ID: true}, &t1.ID); len(got) != 1 {
		t.Fatalf("single theme falls back to all: %v", got)
	}
}

func TestNormalizeCaption(t *testing.T) {
	s := func(v string) *string { return &v }
	if c, err := NormalizeCaption(s("   ")); err != nil || c != nil {
		t.Fatal("blank -> nil")
	}
	if c, _ := NormalizeCaption(s(" 안녕 ")); *c != "안녕" {
		t.Fatal("trimmed")
	}
	if _, err := NormalizeCaption(s(strings.Repeat("가", 201))); !apperr.Is(err, apperr.ValidationFailed) {
		t.Fatal("201 runes rejected")
	}
}
