package http

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

type fakeMe struct {
	me  in.Me
	err error
}

func (f fakeMe) Get(context.Context, uuid.UUID) (in.Me, error) { return f.me, f.err }
func (f fakeMe) UpdateNickname(context.Context, uuid.UUID, string) (in.Me, error) {
	return f.me, f.err
}
func (f fakeMe) RegisterDevice(context.Context, uuid.UUID, string, string) error { return f.err }

type fakeCouple struct {
	cv  in.CoupleView
	err error
}

func (f fakeCouple) Get(context.Context, uuid.UUID) (in.CoupleView, error) { return f.cv, f.err }
func (f fakeCouple) CreateInvite(context.Context, uuid.UUID) (in.Invite, error) {
	return in.Invite{Code: "K7QM2XPA", ExpiresAt: testTime}, f.err
}
func (f fakeCouple) Join(context.Context, uuid.UUID, string) (in.CoupleView, error) {
	return f.cv, f.err
}

func TestMeHandler_GetWithNulls(t *testing.T) {
	r := newV1Router(NewMeHandler(fakeMe{me: in.Me{User: sampleProfile(), FilmBalance: 24}}, &fakeAuth{}, discard))

	rec := do(t, r, call{method: http.MethodGet, path: "/v1/me", token: "good"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"film_balance":24`, `"couple":null`, `"current_date":null`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s missing %s", body, want)
		}
	}
}

func TestMeHandler_RequiresAuth(t *testing.T) {
	r := newV1Router(NewMeHandler(fakeMe{}, &fakeAuth{}, discard))
	wantError(t, do(t, r, call{method: http.MethodGet, path: "/v1/me"}), 401, apperr.Unauthorized)
}

func TestMeHandler_Device(t *testing.T) {
	r := newV1Router(NewMeHandler(fakeMe{}, &fakeAuth{}, discard))
	rec := do(t, r, call{method: http.MethodPut, path: "/v1/me/devices", body: `{"fcm_token":"x","platform":"ios"}`, token: "good"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	bad := newV1Router(NewMeHandler(fakeMe{err: apperr.New(apperr.ValidationFailed, "x")}, &fakeAuth{}, discard))
	wantError(t, do(t, bad, call{method: http.MethodPatch, path: "/v1/me", body: `{"nickname":"x"}`, token: "good"}), 400, apperr.ValidationFailed)
}

func TestCoupleHandler(t *testing.T) {
	cv := in.CoupleView{ID: uuid.New(), Partner: user.Profile{ID: uuid.New(), Nickname: "밥"}, ConnectedAt: testTime}
	r := newV1Router(NewCoupleHandler(fakeCouple{cv: cv}, &fakeAuth{}, discard))

	rec := do(t, r, call{method: http.MethodPost, path: "/v1/couple/invites", token: "good"})
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"code":"K7QM2XPA"`) {
		t.Fatalf("invite: %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, r, call{method: http.MethodPost, path: "/v1/couple/join", body: `{"code":"K7QM2XPA"}`, token: "good"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"nickname":"밥"`) {
		t.Fatalf("join: %d %s", rec.Code, rec.Body.String())
	}

	tests := []struct {
		err    error
		status int
		code   apperr.Code
	}{
		{apperr.New(apperr.CoupleRequired, "x"), 403, apperr.CoupleRequired},
		{apperr.New(apperr.InviteInvalid, "x"), 400, apperr.InviteInvalid},
		{apperr.New(apperr.CoupleAlreadyConnected, "x"), 409, apperr.CoupleAlreadyConnected},
	}
	for _, tt := range tests {
		r := newV1Router(NewCoupleHandler(fakeCouple{err: tt.err}, &fakeAuth{}, discard))
		wantError(t, do(t, r, call{method: http.MethodPost, path: "/v1/couple/join", body: `{"code":"K7QM2XPA"}`, token: "good"}), tt.status, tt.code)
	}
}
