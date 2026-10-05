// Package idtoken 은 Apple/Google ID 토큰을 JWKS 로 검증하는 아웃바운드 어댑터다.
package idtoken

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

const (
	appleJWKSURL  = "https://appleid.apple.com/auth/keys"
	googleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"
	clockLeeway   = 30 * time.Second
)

// Provider 는 한 로그인 수단의 검증 기준이다.
type Provider struct {
	Issuers   []string
	Audiences []string
	// Keyfunc 은 서명 키 조회 함수다. nil 이면 JWKSURL 에서 처음 검증할 때 만든다.
	Keyfunc jwt.Keyfunc
	JWKSURL string
}

// Verifier 는 out.IDTokenVerifier 구현체다.
type Verifier struct {
	ctx       context.Context
	now       func() time.Time
	mu        sync.Mutex
	providers map[user.Provider]*Provider
}

var _ out.IDTokenVerifier = (*Verifier)(nil)

// NewAppleGoogle 은 Apple(aud=번들 ID)과 Google(aud=클라이언트 ID) 검증기를 만든다.
// ctx 는 JWKS 백그라운드 갱신 고루틴의 수명이다.
func NewAppleGoogle(ctx context.Context, appleAudiences, googleAudiences []string) *Verifier {
	return New(ctx, time.Now, map[user.Provider]*Provider{
		user.ProviderApple:  {Issuers: []string{"https://appleid.apple.com"}, Audiences: appleAudiences, JWKSURL: appleJWKSURL},
		user.ProviderGoogle: {Issuers: []string{"https://accounts.google.com", "accounts.google.com"}, Audiences: googleAudiences, JWKSURL: googleJWKSURL},
	})
}

// New 는 검증 기준을 직접 지정해 Verifier 를 만든다.
func New(ctx context.Context, now func() time.Time, providers map[user.Provider]*Provider) *Verifier {
	return &Verifier{ctx: ctx, now: now, providers: providers}
}

type claims struct {
	jwt.RegisteredClaims
	Name string `json:"name"`
}

// Verify 는 서명(JWKS), iss, aud, exp 를 확인하고 subject 와 이름을 반환한다.
func (v *Verifier) Verify(_ context.Context, provider user.Provider, raw string) (out.IDTokenClaims, error) {
	p, kf, err := v.provider(provider)
	if err != nil {
		return out.IDTokenClaims{}, err
	}
	if len(p.Audiences) == 0 {
		return out.IDTokenClaims{}, fmt.Errorf("%s client ids are not configured", provider)
	}
	var c claims
	_, err = jwt.ParseWithClaims(raw, &c, kf,
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(clockLeeway),
		jwt.WithTimeFunc(v.now),
	)
	if err != nil {
		return out.IDTokenClaims{}, fmt.Errorf("%w: %s: %v", out.ErrInvalidIDToken, provider, err)
	}
	if !slices.Contains(p.Issuers, c.Issuer) {
		return out.IDTokenClaims{}, fmt.Errorf("%w: unexpected issuer %q", out.ErrInvalidIDToken, c.Issuer)
	}
	if !audienceAllowed(c.Audience, p.Audiences) {
		return out.IDTokenClaims{}, fmt.Errorf("%w: unexpected audience", out.ErrInvalidIDToken)
	}
	if c.Subject == "" {
		return out.IDTokenClaims{}, fmt.Errorf("%w: missing subject", out.ErrInvalidIDToken)
	}
	return out.IDTokenClaims{Subject: c.Subject, Name: c.Name}, nil
}

// provider 는 검증 기준과 키 조회 함수를 반환한다. JWKS 클라이언트는 처음 쓸 때 만든다.
func (v *Verifier) provider(name user.Provider) (*Provider, jwt.Keyfunc, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	p, ok := v.providers[name]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported provider %s", name)
	}
	if p.Keyfunc == nil {
		kf, err := keyfunc.NewDefaultCtx(v.ctx, []string{p.JWKSURL})
		if err != nil {
			return nil, nil, fmt.Errorf("load %s jwks: %w", name, err)
		}
		p.Keyfunc = kf.Keyfunc
	}
	return p, p.Keyfunc, nil
}

func audienceAllowed(got jwt.ClaimStrings, allowed []string) bool {
	for _, a := range got {
		if slices.Contains(allowed, a) {
			return true
		}
	}
	return false
}
