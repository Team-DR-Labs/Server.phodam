package service

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

func TestAdmin_GrantFilm(t *testing.T) {
	h := newHarness(t)
	a := h.newUser("alice", 0)
	svc := NewAdminService(h.deps)

	balance, err := svc.GrantFilm(ctx, a, 24, ptr("테스트"))
	mustOK(t, err)

	last := h.st.ledger[len(h.st.ledger)-1]
	if balance != 24 || h.st.ledgerSum(a) != 24 || last.Reason != user.LedgerAdminGrant || *last.Memo != "테스트" {
		t.Fatalf("balance=%d ledger=%d entry=%+v", balance, h.st.ledgerSum(a), last)
	}

	tests := []struct {
		name   string
		user   uuid.UUID
		shots  int
		reason *string
		want   apperr.Code
	}{
		{"zero shots", a, 0, nil, apperr.ValidationFailed},
		{"too many", a, 241, nil, apperr.ValidationFailed},
		{"long reason", a, 1, ptr(strings.Repeat("가", 101)), apperr.ValidationFailed},
		{"unknown user", uuid.New(), 1, nil, apperr.NotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GrantFilm(ctx, tt.user, tt.shots, tt.reason)
			wantCode(t, err, tt.want)
		})
	}
}

func TestAdmin_GrantedFilmAllowsShotAfterExhaustion(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()
	dateID := h.startJoined(t, a, b)
	h.st.users[a] = withBalance(h.st.users[a], 0)
	_, err := NewPhotoService(h.deps).ReserveShot(ctx, a, dateID)
	wantCode(t, err, apperr.FilmExhausted)

	_, err = NewAdminService(h.deps).GrantFilm(ctx, a, 1, nil)
	mustOK(t, err)
	res, err := NewPhotoService(h.deps).ReserveShot(ctx, a, dateID)
	mustOK(t, err)
	if res.FilmBalance != 0 {
		t.Fatalf("balance = %d", res.FilmBalance)
	}
}
