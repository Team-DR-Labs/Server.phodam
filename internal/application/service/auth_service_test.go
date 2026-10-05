package service

import (
	"strings"
	"testing"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

func ptr[T any](v T) *T { return &v }

func TestAuth_DevLoginCreatesUserWithSignupFilm(t *testing.T) {
	h := newHarness(t)
	svc := NewAuthService(h.deps)

	first, err := svc.LoginDev(ctx, "alice", ptr("앨리스"))
	mustOK(t, err)
	if !first.IsNewUser || first.User.Nickname != "앨리스" {
		t.Fatalf("unexpected first login: %+v", first)
	}
	u := h.st.users[first.User.ID]
	if u.FilmBalance != user.SignupFilm || h.st.ledgerSum(u.ID) != user.SignupFilm {
		t.Fatalf("film = %d, ledger = %d, want 24", u.FilmBalance, h.st.ledgerSum(u.ID))
	}

	again, err := svc.LoginDev(ctx, "alice", nil)
	mustOK(t, err)
	if again.IsNewUser || again.User.ID != first.User.ID {
		t.Fatalf("second login should reuse user: %+v", again)
	}
	if h.st.ledgerSum(u.ID) != user.SignupFilm {
		t.Fatal("signup film must be granted once")
	}
}

func TestAuth_NicknameResolution(t *testing.T) {
	h := newHarness(t)
	h.deps.IDTokens = fakeVerifier{claims: map[string]out.IDTokenClaims{
		"g-named":   {Subject: "g1", Name: "구글이름"},
		"g-unnamed": {Subject: "g2"},
		"apple":     {Subject: "a1"},
	}}
	svc := NewAuthService(h.deps)

	tests := []struct {
		name string
		run  func() (string, error)
		want string
	}{
		{"token name first", func() (string, error) { r, err := svc.LoginGoogle(ctx, "g-named"); return r.User.Nickname, err }, "구글이름"},
		{"default", func() (string, error) { r, err := svc.LoginGoogle(ctx, "g-unnamed"); return r.User.Nickname, err }, user.DefaultNickname},
		{"request nickname", func() (string, error) {
			r, err := svc.LoginApple(ctx, "apple", ptr(" 사과 "))
			return r.User.Nickname, err
		}, "사과"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.run()
			mustOK(t, err)
			if got != tt.want {
				t.Fatalf("nickname = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAuth_Errors(t *testing.T) {
	h := newHarness(t)
	svc := NewAuthService(h.deps)

	_, err := svc.LoginApple(ctx, "forged", nil)
	wantCode(t, err, apperr.AuthInvalidIDToken)
	_, err = svc.LoginGoogle(ctx, "config-error")
	if _, isDomain := apperr.As(err); err == nil || isDomain {
		t.Fatalf("server-side verifier failure must be internal, got %v", err)
	}
	_, err = svc.LoginDev(ctx, "", nil)
	wantCode(t, err, apperr.ValidationFailed)
	_, err = svc.LoginDev(ctx, "bob", ptr(strings.Repeat("가", 21)))
	wantCode(t, err, apperr.ValidationFailed)
	_, err = svc.Authenticate(ctx, "garbage")
	wantCode(t, err, apperr.Unauthorized)
}

func TestAuth_RefreshRotatesAndLogoutRevokes(t *testing.T) {
	h := newHarness(t)
	svc := NewAuthService(h.deps)
	login, err := svc.LoginDev(ctx, "alice", nil)
	mustOK(t, err)

	rotated, err := svc.Refresh(ctx, login.RefreshToken)
	mustOK(t, err)
	if rotated.RefreshToken == login.RefreshToken || rotated.User.ID != login.User.ID {
		t.Fatalf("refresh must rotate token: %+v", rotated)
	}
	_, err = svc.Refresh(ctx, login.RefreshToken)
	wantCode(t, err, apperr.AuthInvalidRefreshToken)

	mustOK(t, svc.Logout(ctx, login.User.ID, rotated.RefreshToken))
	mustOK(t, svc.Logout(ctx, login.User.ID, rotated.RefreshToken))
	_, err = svc.Refresh(ctx, rotated.RefreshToken)
	wantCode(t, err, apperr.AuthInvalidRefreshToken)
}

func TestAuth_RefreshExpires(t *testing.T) {
	h := newHarness(t)
	svc := NewAuthService(h.deps)
	login, err := svc.LoginDev(ctx, "alice", nil)
	mustOK(t, err)

	h.clock.Advance(RefreshTokenTTL)
	_, err = svc.Refresh(ctx, login.RefreshToken)
	wantCode(t, err, apperr.AuthInvalidRefreshToken)
}
