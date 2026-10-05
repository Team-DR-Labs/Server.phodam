package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestJWTIssuer(t *testing.T) {
	now := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	iss := NewJWTIssuer(strings.Repeat("k", 32))
	id := uuid.New()

	tok, exp, err := iss.Issue(id, now)
	if err != nil || !exp.Equal(now.Add(time.Hour)) {
		t.Fatalf("issue: %v, exp=%v", err, exp)
	}

	tests := []struct {
		name    string
		token   string
		at      time.Time
		wantErr bool
	}{
		{"valid", tok, now.Add(59 * time.Minute), false},
		{"expired", tok, now.Add(time.Hour + time.Second), true},
		{"other secret", mustSign(t, "other-secret-other-secret-other!!", id, now), now, true},
		{"none alg", noneToken(t, id, now), now, true},
		{"garbage", "abc.def.ghi", now, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := iss.Parse(tt.token, tt.at)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || got != id {
				t.Fatalf("parse = %v, %v", got, err)
			}
		})
	}
}

func mustSign(t *testing.T, secret string, id uuid.UUID, now time.Time) string {
	t.Helper()
	tok, _, err := NewJWTIssuer(secret).Issue(id, now)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func noneToken(t *testing.T, id uuid.UUID, now time.Time) string {
	t.Helper()
	claims := jwt.RegisteredClaims{Issuer: issuer, Subject: id.String(), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}
	s, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
