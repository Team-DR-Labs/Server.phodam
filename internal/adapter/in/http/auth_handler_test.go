package http

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
)

func TestAuthHandler_DevLogin(t *testing.T) {
	uc := &fakeAuth{login: in.AuthResult{
		AccessToken: "a", AccessTokenExpiresAt: testTime, RefreshToken: "r",
		RefreshTokenExpiresAt: testTime, IsNewUser: true, User: sampleProfile(),
	}}
	r := newV1Router(NewAuthHandler(uc, discard, true))

	rec := do(t, r, call{method: http.MethodPost, path: "/v1/auth/dev", body: `{"dev_id":"alice"}`})

	if rec.Code != http.StatusOK || uc.gotDev != "alice" {
		t.Fatalf("status = %d, dev = %q", rec.Code, uc.gotDev)
	}
	body := rec.Body.String()
	for _, want := range []string{`"access_token":"a"`, `"access_token_expires_at":"2026-10-05T03:00:00Z"`, `"is_new_user":true`, `"nickname":"앨리스"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s missing %s", body, want)
		}
	}
}

func TestAuthHandler_DevRouteOnlyWhenEnabled(t *testing.T) {
	r := newV1Router(NewAuthHandler(&fakeAuth{}, discard, false))
	rec := do(t, r, call{method: http.MethodPost, path: "/v1/auth/dev", body: `{"dev_id":"alice"}`})
	wantError(t, rec, http.StatusNotFound, apperr.NotFound)
}

func TestAuthHandler_Errors(t *testing.T) {
	tests := []struct {
		name   string
		uc     *fakeAuth
		call   call
		status int
		code   apperr.Code
	}{
		{"missing field", &fakeAuth{}, call{method: http.MethodPost, path: "/v1/auth/google", body: `{}`}, 400, apperr.ValidationFailed},
		{"bad json", &fakeAuth{}, call{method: http.MethodPost, path: "/v1/auth/apple", body: `{`}, 400, apperr.ValidationFailed},
		{"invalid id token", &fakeAuth{err: apperr.New(apperr.AuthInvalidIDToken, "x")}, call{method: http.MethodPost, path: "/v1/auth/apple", body: `{"identity_token":"t"}`}, 401, apperr.AuthInvalidIDToken},
		{"invalid refresh", &fakeAuth{err: apperr.New(apperr.AuthInvalidRefreshToken, "x")}, call{method: http.MethodPost, path: "/v1/auth/refresh", body: `{"refresh_token":"t"}`}, 401, apperr.AuthInvalidRefreshToken},
		{"logout without bearer", &fakeAuth{}, call{method: http.MethodPost, path: "/v1/auth/logout", body: `{"refresh_token":"t"}`}, 401, apperr.Unauthorized},
		{"logout bad bearer", &fakeAuth{}, call{method: http.MethodPost, path: "/v1/auth/logout", body: `{"refresh_token":"t"}`, token: "bad"}, 401, apperr.Unauthorized},
		{"internal hidden", &fakeAuth{err: errSecret}, call{method: http.MethodPost, path: "/v1/auth/google", body: `{"id_token":"t"}`}, 500, apperr.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newV1Router(NewAuthHandler(tt.uc, discard, true)), tt.call)
			wantError(t, rec, tt.status, tt.code)
			if strings.Contains(rec.Body.String(), "secret") {
				t.Fatal("internal error detail leaked")
			}
		})
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	r := newV1Router(NewAuthHandler(&fakeAuth{}, discard, true))
	rec := do(t, r, call{method: http.MethodPost, path: "/v1/auth/logout", body: `{"refresh_token":"t"}`, token: "good"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
