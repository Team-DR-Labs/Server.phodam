package service

import (
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
)

func TestDiary_VisibilityRules(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	ds := NewDiaryService(h.deps)
	dateID := h.startJoined(t, a, b)

	_, err := ds.Get(ctx, a, dateID)
	wantCode(t, err, apperr.NotFound)

	h.submit(t, a, dateID, h.shoot(t, a, dateID), "a의 글")
	d, err := ds.Get(ctx, a, dateID)
	mustOK(t, err)
	if d.Visibility != diary.VisibilityWaiting || len(d.Entries) != 1 || !d.Entries[0].IsMe {
		t.Fatalf("waiting shows only mine: %+v", d)
	}
	_, err = ds.Get(ctx, b, dateID)
	wantCode(t, err, apperr.NotFound)
	page, err := ds.List(ctx, b, "", 0)
	mustOK(t, err)
	if len(page.Items) != 0 {
		t.Fatal("partner who has not submitted sees nothing")
	}

	h.submit(t, b, dateID, h.shoot(t, b, dateID), "")
	d, err = ds.Get(ctx, b, dateID)
	mustOK(t, err)
	if d.Visibility != diary.VisibilityShared || len(d.Entries) != 2 || !d.Entries[0].IsMe || d.Entries[1].Author.ID != a {
		t.Fatalf("shared shows both, mine first: %+v", d)
	}
	if d.Entries[1].Caption == nil || *d.Entries[1].Caption != "a의 글" || d.Entries[0].Caption != nil {
		t.Fatalf("captions: %+v", d.Entries)
	}
	if d.LocalDate != "2026-10-05" {
		t.Fatalf("local_date = %s", d.LocalDate)
	}
}

func TestDiary_ExpiredWithOneSubmissionIsPrivate(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	ds := NewDiaryService(h.deps)
	dateID := h.startJoined(t, a, b)
	h.submit(t, a, dateID, h.shoot(t, a, dateID), "")

	h.clock.Advance(dating.SubmitWindow)
	d, err := ds.Get(ctx, a, dateID)
	mustOK(t, err)
	if d.Visibility != diary.VisibilityPrivate || len(d.Entries) != 1 {
		t.Fatalf("expired -> private, mine only: %+v", d)
	}
	_, err = ds.Get(ctx, b, dateID)
	wantCode(t, err, apperr.NotFound)
}

func TestDiary_ListOrderAndCursor(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	ds := NewDiaryService(h.deps)
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		dateID := h.startJoined(t, a, b)
		h.submit(t, a, dateID, h.shoot(t, a, dateID), "")
		h.submit(t, b, dateID, h.shoot(t, b, dateID), "")
		ids = append(ids, dateID)
		h.clock.Advance(dating.SubmitWindow)
	}

	first, err := ds.List(ctx, a, "", 2)
	mustOK(t, err)
	if len(first.Items) != 2 || first.Items[0].DateID != ids[2] || first.Items[1].DateID != ids[1] || first.NextCursor == nil {
		t.Fatalf("first page: %+v", first)
	}
	if first.Items[0].Thumbnail.URL == "" || first.Items[0].Visibility != diary.VisibilityShared {
		t.Fatalf("item: %+v", first.Items[0])
	}
	second, err := ds.List(ctx, a, *first.NextCursor, 2)
	mustOK(t, err)
	if len(second.Items) != 1 || second.Items[0].DateID != ids[0] || second.NextCursor != nil {
		t.Fatalf("second page: %+v", second)
	}

	_, err = ds.List(ctx, a, "%%%", 2)
	wantCode(t, err, apperr.ValidationFailed)
	_, err = ds.List(ctx, a, "", 51)
	wantCode(t, err, apperr.ValidationFailed)
	_, err = ds.List(ctx, h.newUser("carol", 24), "", 0)
	wantCode(t, err, apperr.CoupleRequired)
}
