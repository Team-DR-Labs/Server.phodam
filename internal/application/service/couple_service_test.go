package service

import (
	"strings"
	"testing"
	"time"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
)

func TestCouple_InviteAndJoin(t *testing.T) {
	h := newHarness(t)
	svc := NewCoupleService(h.deps)
	a, b := h.newUser("alice", 24), h.newUser("bob", 24)

	_, err := svc.Get(ctx, a)
	wantCode(t, err, apperr.CoupleRequired)

	inv, err := svc.CreateInvite(ctx, a)
	mustOK(t, err)
	if len(inv.Code) != 8 || strings.ContainsAny(inv.Code, "01ILO") || !inv.ExpiresAt.Equal(baseTime.Add(couple.InviteTTL)) {
		t.Fatalf("unexpected invite: %+v", inv)
	}

	_, err = svc.Join(ctx, a, inv.Code)
	wantCode(t, err, apperr.InviteInvalid)

	cv, err := svc.Join(ctx, b, strings.ToLower(inv.Code))
	mustOK(t, err)
	if cv.Partner.ID != a {
		t.Fatalf("partner = %v, want alice", cv.Partner.ID)
	}
	got, err := svc.Get(ctx, a)
	mustOK(t, err)
	if got.Partner.ID != b {
		t.Fatalf("alice partner = %v, want bob", got.Partner.ID)
	}

	_, err = svc.CreateInvite(ctx, a)
	wantCode(t, err, apperr.CoupleAlreadyConnected)
	_, err = svc.Join(ctx, b, inv.Code)
	wantCode(t, err, apperr.CoupleAlreadyConnected)
}

func TestCouple_JoinRejections(t *testing.T) {
	tests := []struct {
		name string
		prep func(h *harness, svc *CoupleService) (joiner string, code string)
		want apperr.Code
	}{
		{"unknown code", func(h *harness, _ *CoupleService) (string, string) { return "bob", "ABCDEFGH" }, apperr.InviteInvalid},
		{"malformed", func(h *harness, _ *CoupleService) (string, string) { return "bob", "abc" }, apperr.ValidationFailed},
		{"expired", func(h *harness, svc *CoupleService) (string, string) {
			inv, _ := svc.CreateInvite(ctx, h.userByNick("alice"))
			h.clock.Advance(couple.InviteTTL + time.Second)
			return "bob", inv.Code
		}, apperr.InviteInvalid},
		{"replaced by newer code", func(h *harness, svc *CoupleService) (string, string) {
			old, _ := svc.CreateInvite(ctx, h.userByNick("alice"))
			_, _ = svc.CreateInvite(ctx, h.userByNick("alice"))
			return "bob", old.Code
		}, apperr.InviteInvalid},
		{"used once", func(h *harness, svc *CoupleService) (string, string) {
			inv, _ := svc.CreateInvite(ctx, h.userByNick("alice"))
			_, _ = svc.Join(ctx, h.userByNick("bob"), inv.Code)
			return "carol", inv.Code
		}, apperr.InviteInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			for _, n := range []string{"alice", "bob", "carol"} {
				h.newUser(n, 24)
			}
			svc := NewCoupleService(h.deps)
			joiner, code := tt.prep(h, svc)
			_, err := svc.Join(ctx, h.userByNick(joiner), code)
			wantCode(t, err, tt.want)
		})
	}
}

func TestCouple_JoinWhenCreatorAlreadyConnected(t *testing.T) {
	h := newHarness(t)
	svc := NewCoupleService(h.deps)
	a, _ := h.newCouple()
	c := h.newUser("carol", 24)
	// 연결 전에 만든 코드가 남아 있는 상황을 만든다.
	h.st.invites["ZZZZZZZZ"] = couple.Invite{Code: "ZZZZZZZZ", CreatorID: a, ExpiresAt: baseTime.Add(time.Hour)}

	_, err := svc.Join(ctx, c, "ZZZZZZZZ")
	wantCode(t, err, apperr.CoupleAlreadyConnected)
}
