// Package token 은 자체 액세스 토큰(HS256 JWT) 아웃바운드 어댑터다.
package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
)

const (
	// AccessTTL 은 액세스 토큰 유효 시간이다.
	AccessTTL = time.Hour
	issuer    = "phodam"
)

// JWTIssuer 는 out.AccessTokenIssuer 구현체다.
type JWTIssuer struct {
	secret []byte
}

var _ out.AccessTokenIssuer = (*JWTIssuer)(nil)

// NewJWTIssuer 는 HS256 서명 키로 JWTIssuer 를 생성한다.
func NewJWTIssuer(secret string) *JWTIssuer { return &JWTIssuer{secret: []byte(secret)} }

// Issue 는 sub=userID 인 액세스 토큰을 발급한다.
func (j *JWTIssuer) Issue(userID uuid.UUID, now time.Time) (string, time.Time, error) {
	exp := now.Add(AccessTTL)
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, exp, nil
}

// Parse 는 서명·발급자·만료를 검증하고 사용자 ID 를 반환한다.
func (j *JWTIssuer) Parse(raw string, now time.Time) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(raw, &claims, func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse access token: %w", err)
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errors.New("access token subject is not a uuid")
	}
	return id, nil
}
