package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

const (
	// RefreshTokenTTL 은 리프레시 토큰 유효 기간이다.
	RefreshTokenTTL   = 30 * 24 * time.Hour
	refreshTokenBytes = 32
	devIDMaxLen       = 40
)

// AuthService 는 AuthUseCase 구현체다.
type AuthService struct{ d Deps }

var _ in.AuthUseCase = (*AuthService)(nil)

// NewAuthService 는 AuthService 를 생성한다.
func NewAuthService(d Deps) *AuthService { return &AuthService{d: d} }

// LoginApple 은 Apple identityToken 으로 로그인한다.
func (s *AuthService) LoginApple(ctx context.Context, identityToken string, nickname *string) (in.AuthResult, error) {
	requested, err := user.OptionalNickname(nickname)
	if err != nil {
		return in.AuthResult{}, err
	}
	claims, err := s.verify(ctx, user.ProviderApple, identityToken)
	if err != nil {
		return in.AuthResult{}, err
	}
	return s.login(ctx, user.ProviderApple, claims.Subject, claims.Name, requested)
}

// LoginGoogle 은 Google ID 토큰으로 로그인한다.
func (s *AuthService) LoginGoogle(ctx context.Context, idToken string) (in.AuthResult, error) {
	claims, err := s.verify(ctx, user.ProviderGoogle, idToken)
	if err != nil {
		return in.AuthResult{}, err
	}
	return s.login(ctx, user.ProviderGoogle, claims.Subject, claims.Name, "")
}

// LoginDev 는 개발용 로그인이다. 같은 devID 는 같은 사용자다.
func (s *AuthService) LoginDev(ctx context.Context, devID string, nickname *string) (in.AuthResult, error) {
	id := strings.TrimSpace(devID)
	if id == "" || len(id) > devIDMaxLen {
		return in.AuthResult{}, errValidation("dev_id must be 1-40 characters")
	}
	requested, err := user.OptionalNickname(nickname)
	if err != nil {
		return in.AuthResult{}, err
	}
	return s.login(ctx, user.ProviderDev, id, "", requested)
}

func (s *AuthService) verify(ctx context.Context, p user.Provider, token string) (out.IDTokenClaims, error) {
	if strings.TrimSpace(token) == "" {
		return out.IDTokenClaims{}, errValidation("id token is required")
	}
	claims, err := s.d.IDTokens.Verify(ctx, p, token)
	if err != nil || claims.Subject == "" {
		return out.IDTokenClaims{}, apperr.New(apperr.AuthInvalidIDToken, "invalid id token")
	}
	return claims, nil
}

// login 은 식별자로 사용자를 찾거나 만들고(필름 24장 지급) 토큰을 발급한다.
func (s *AuthService) login(ctx context.Context, p user.Provider, subject, tokenName, requested string) (in.AuthResult, error) {
	res, err := s.loginTx(ctx, p, subject, tokenName, requested)
	if errors.Is(err, out.ErrDuplicate) {
		// 같은 식별자로 동시에 첫 로그인한 경우: 먼저 만든 사용자로 로그인한다.
		res, err = s.loginTx(ctx, p, subject, tokenName, requested)
	}
	return res, err
}

func (s *AuthService) loginTx(ctx context.Context, p user.Provider, subject, tokenName, requested string) (in.AuthResult, error) {
	var res in.AuthResult
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		u, isNew, err := s.findOrCreate(ctx, p, subject, tokenName, requested)
		if err != nil {
			return err
		}
		res, err = s.issue(ctx, u, isNew)
		return err
	})
	return res, err
}

func (s *AuthService) findOrCreate(ctx context.Context, p user.Provider, subject, tokenName, requested string) (user.User, bool, error) {
	userID, err := s.d.Identities.FindUserID(ctx, p, subject)
	if err == nil {
		u, err := s.d.Users.Get(ctx, userID)
		if err != nil {
			return user.User{}, false, fmt.Errorf("get user: %w", err)
		}
		return u, false, nil
	}
	if !errors.Is(err, out.ErrNotFound) {
		return user.User{}, false, fmt.Errorf("find identity: %w", err)
	}

	now := s.d.now()
	u := user.New(uuid.New(), user.ResolveNickname(tokenName, requested), now)
	if err := s.d.Users.Create(ctx, u); err != nil {
		return user.User{}, false, fmt.Errorf("create user: %w", err)
	}
	if err := s.d.Identities.Create(ctx, user.Identity{UserID: u.ID, Provider: p, Subject: subject}); err != nil {
		return user.User{}, false, err
	}
	entry := user.LedgerEntry{ID: uuid.New(), UserID: u.ID, Delta: user.SignupFilm, Reason: user.LedgerSignup, CreatedAt: now}
	if err := s.d.Ledger.Append(ctx, entry); err != nil {
		return user.User{}, false, fmt.Errorf("append ledger: %w", err)
	}
	return u, true, nil
}

// Refresh 는 리프레시 토큰을 회전한다. 이전 토큰은 폐기된다.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (in.AuthResult, error) {
	invalid := apperr.New(apperr.AuthInvalidRefreshToken, "invalid refresh token")
	if strings.TrimSpace(refreshToken) == "" {
		return in.AuthResult{}, invalid
	}
	var res in.AuthResult
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		userID, err := s.d.Refresh.Consume(ctx, hashToken(refreshToken), s.d.now())
		if errors.Is(err, out.ErrNotFound) {
			return invalid
		}
		if err != nil {
			return fmt.Errorf("consume refresh token: %w", err)
		}
		u, err := s.d.Users.Get(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		res, err = s.issue(ctx, u, false)
		return err
	})
	return res, err
}

// Logout 은 요청자 소유의 리프레시 토큰을 폐기한다. 이미 폐기됐거나 없어도 성공이다.
func (s *AuthService) Logout(ctx context.Context, userID uuid.UUID, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return errValidation("refresh_token is required")
	}
	if err := s.d.Refresh.Revoke(ctx, userID, hashToken(refreshToken), s.d.now()); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// Authenticate 는 액세스 토큰을 검증한다.
func (s *AuthService) Authenticate(_ context.Context, accessToken string) (uuid.UUID, error) {
	id, err := s.d.Access.Parse(accessToken, s.d.now())
	if err != nil {
		return uuid.Nil, apperr.New(apperr.Unauthorized, "invalid access token")
	}
	return id, nil
}

func (s *AuthService) issue(ctx context.Context, u user.User, isNew bool) (in.AuthResult, error) {
	now := s.d.now()
	access, accessExp, err := s.d.Access.Issue(u.ID, now)
	if err != nil {
		return in.AuthResult{}, fmt.Errorf("issue access token: %w", err)
	}
	raw, err := newRefreshToken()
	if err != nil {
		return in.AuthResult{}, err
	}
	rt := out.RefreshToken{
		ID: uuid.New(), UserID: u.ID, TokenHash: hashToken(raw),
		ExpiresAt: now.Add(RefreshTokenTTL), CreatedAt: now,
	}
	if err := s.d.Refresh.Create(ctx, rt); err != nil {
		return in.AuthResult{}, fmt.Errorf("store refresh token: %w", err)
	}
	return in.AuthResult{
		AccessToken: access, AccessTokenExpiresAt: accessExp,
		RefreshToken: raw, RefreshTokenExpiresAt: rt.ExpiresAt,
		IsNewUser: isNew, User: u.Profile(),
	}, nil
}

func newRefreshToken() (string, error) {
	b := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
