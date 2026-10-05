// Package user 는 사용자, 로그인 식별자, 필름 원장, 기기 토큰 도메인 모델을 정의한다.
package user

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
)

const (
	// SignupFilm 은 가입 시 지급하는 필름 매수다.
	SignupFilm = 24
	// DefaultNickname 은 토큰과 요청 모두에 이름이 없을 때 쓰는 닉네임이다.
	DefaultNickname = "포담 사용자"
	// NicknameMaxRunes 는 닉네임 최대 길이(유니코드 문자 수)다.
	NicknameMaxRunes = 20
	// StatusActive 는 MVP 에서 쓰는 유일한 사용자 상태다.
	StatusActive = "active"
)

// Provider 는 로그인 수단이다.
type Provider string

const (
	ProviderApple  Provider = "apple"
	ProviderGoogle Provider = "google"
	ProviderDev    Provider = "dev"
)

// User 는 사용자 엔티티다.
type User struct {
	ID          uuid.UUID
	Nickname    string
	FilmBalance int
	Status      string
	CreatedAt   time.Time
}

// Profile 은 다른 사용자에게 보여 주는 최소 정보다.
type Profile struct {
	ID       uuid.UUID
	Nickname string
}

// Profile 은 사용자의 공개 프로필을 반환한다.
func (u User) Profile() Profile { return Profile{ID: u.ID, Nickname: u.Nickname} }

// New 는 첫 로그인 사용자를 만든다. 필름 잔액은 원장 기록과 함께 SignupFilm 으로 시작한다.
func New(id uuid.UUID, nickname string, now time.Time) User {
	return User{ID: id, Nickname: nickname, FilmBalance: SignupFilm, Status: StatusActive, CreatedAt: now}
}

// Identity 는 외부 로그인 식별자 (provider, subject) 다.
type Identity struct {
	UserID   uuid.UUID
	Provider Provider
	Subject  string
}

// ValidateNickname 은 PATCH /me 의 닉네임을 정리하고 검증한다 (앞뒤 공백 제거 후 1~20자).
func ValidateNickname(raw string) (string, error) {
	n := strings.TrimSpace(raw)
	if n == "" || utf8.RuneCountInString(n) > NicknameMaxRunes {
		return "", apperr.New(apperr.ValidationFailed, "nickname must be 1-20 characters")
	}
	return n, nil
}

// OptionalNickname 은 로그인 요청의 선택 닉네임을 검증한다. 비어 있으면 "" 를 반환한다.
func OptionalNickname(raw *string) (string, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", nil
	}
	return ValidateNickname(*raw)
}

// ResolveNickname 은 토큰 이름 → 요청 닉네임 → 기본값 순으로 닉네임을 정한다.
// 토큰 이름이 길면 20자로 자른다.
func ResolveNickname(tokenName, requested string) string {
	if n := strings.TrimSpace(tokenName); n != "" {
		return truncateRunes(n, NicknameMaxRunes)
	}
	if requested != "" {
		return requested
	}
	return DefaultNickname
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// LedgerReason 은 필름 원장 사유다.
type LedgerReason string

const (
	LedgerSignup     LedgerReason = "signup"
	LedgerShot       LedgerReason = "shot"
	LedgerAdminGrant LedgerReason = "admin_grant"
)

// LedgerEntry 는 필름 증감 기록이다. 잔액(users.film_balance)은 항상 원장 합계와 같다.
type LedgerEntry struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Delta     int
	Reason    LedgerReason
	RefID     *uuid.UUID
	Memo      *string
	CreatedAt time.Time
}

// Platform 은 기기 플랫폼이다.
type Platform string

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

// Device 는 FCM 기기 토큰 등록 정보다.
type Device struct {
	UserID   uuid.UUID
	Token    string
	Platform Platform
}

// fcmTokenMaxLen 은 비정상적으로 긴 토큰을 거르기 위한 상한이다.
const fcmTokenMaxLen = 4096

// NewDevice 는 기기 등록 요청을 검증한다.
func NewDevice(userID uuid.UUID, token, platform string) (Device, error) {
	t := strings.TrimSpace(token)
	if t == "" || len(t) > fcmTokenMaxLen {
		return Device{}, apperr.New(apperr.ValidationFailed, "fcm_token is invalid")
	}
	p := Platform(platform)
	if p != PlatformIOS && p != PlatformAndroid {
		return Device{}, apperr.New(apperr.ValidationFailed, "platform must be ios or android")
	}
	return Device{UserID: userID, Token: t, Platform: p}, nil
}
