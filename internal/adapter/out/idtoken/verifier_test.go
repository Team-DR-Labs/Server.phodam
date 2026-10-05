package idtoken

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

func TestVerifier(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	v := New(context.Background(), func() time.Time { return now }, map[user.Provider]*Provider{
		user.ProviderGoogle: {
			Issuers: []string{"https://accounts.google.com"}, Audiences: []string{"client-1"},
			Keyfunc: func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
		},
	})

	sign := func(k *rsa.PrivateKey, iss, aud string, exp time.Time) string {
		c := claims{Name: "구글", RegisteredClaims: jwt.RegisteredClaims{
			Issuer: iss, Subject: "sub-1", Audience: jwt.ClaimStrings{aud}, ExpiresAt: jwt.NewNumericDate(exp),
		}}
		s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, c).SignedString(k)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	good := "https://accounts.google.com"
	tests := []struct {
		name     string
		provider user.Provider
		token    string
		wantErr  bool
	}{
		{"valid", user.ProviderGoogle, sign(key, good, "client-1", now.Add(time.Hour)), false},
		{"wrong audience", user.ProviderGoogle, sign(key, good, "client-2", now.Add(time.Hour)), true},
		{"wrong issuer", user.ProviderGoogle, sign(key, "https://evil", "client-1", now.Add(time.Hour)), true},
		{"expired", user.ProviderGoogle, sign(key, good, "client-1", now.Add(-time.Hour)), true},
		{"wrong key", user.ProviderGoogle, sign(other, good, "client-1", now.Add(time.Hour)), true},
		{"unknown provider", user.ProviderApple, sign(key, good, "client-1", now.Add(time.Hour)), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v.Verify(context.Background(), tt.provider, tt.token)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || got.Subject != "sub-1" || got.Name != "구글" {
				t.Fatalf("verify = %+v, %v", got, err)
			}
		})
	}
}

func TestVerifier_RejectsWithoutConfiguredAudience(t *testing.T) {
	v := New(context.Background(), time.Now, map[user.Provider]*Provider{
		user.ProviderApple: {Issuers: []string{"https://appleid.apple.com"}, Keyfunc: func(*jwt.Token) (any, error) { return nil, nil }},
	})
	if _, err := v.Verify(context.Background(), user.ProviderApple, "x.y.z"); err == nil {
		t.Fatal("expected error when client ids are empty")
	}
}
