package service

import (
	"strings"
	"testing"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
)

func TestMe_GetWithoutCouple(t *testing.T) {
	h := newHarness(t)
	a := h.newUser("alice", 24)

	me, err := NewMeService(h.deps).Get(ctx, a)
	mustOK(t, err)
	if me.FilmBalance != 24 || me.Couple != nil || me.CurrentDate != nil {
		t.Fatalf("unexpected me: %+v", me)
	}
}

func TestMe_GetWithCouple(t *testing.T) {
	h := newHarness(t)
	a, b := h.newCouple()

	me, err := NewMeService(h.deps).Get(ctx, a)
	mustOK(t, err)
	if me.Couple == nil || me.Couple.Partner.ID != b {
		t.Fatalf("couple partner should be bob: %+v", me.Couple)
	}
}

func TestMe_UpdateNickname(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		code  apperr.Code
	}{
		{name: "trimmed", input: "  새이름 ", want: "새이름"},
		{name: "20 runes ok", input: strings.Repeat("가", 20), want: strings.Repeat("가", 20)},
		{name: "empty", input: "   ", code: apperr.ValidationFailed},
		{name: "21 runes", input: strings.Repeat("가", 21), code: apperr.ValidationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			a := h.newUser("alice", 24)
			me, err := NewMeService(h.deps).UpdateNickname(ctx, a, tt.input)
			if tt.code != "" {
				wantCode(t, err, tt.code)
				return
			}
			mustOK(t, err)
			if me.User.Nickname != tt.want {
				t.Fatalf("nickname = %q, want %q", me.User.Nickname, tt.want)
			}
		})
	}
}

func TestMe_RegisterDeviceMovesOwner(t *testing.T) {
	h := newHarness(t)
	svc := NewMeService(h.deps)
	a, b := h.newUser("alice", 24), h.newUser("bob", 24)

	mustOK(t, svc.RegisterDevice(ctx, a, "shared-token", "ios"))
	mustOK(t, svc.RegisterDevice(ctx, b, "shared-token", "android"))
	if got := h.st.devices["shared-token"]; got.UserID != b || got.Platform != "android" {
		t.Fatalf("token owner should move to bob: %+v", got)
	}
	wantCode(t, svc.RegisterDevice(ctx, a, "t", "windows"), apperr.ValidationFailed)
	wantCode(t, svc.RegisterDevice(ctx, a, " ", "ios"), apperr.ValidationFailed)
}
