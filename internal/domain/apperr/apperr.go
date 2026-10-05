// Package apperr 는 정책 §13 의 에러 코드를 담는 도메인 에러를 정의한다.
// HTTP 상태 코드로의 변환은 인바운드 어댑터의 단일 매퍼가 맡는다.
package apperr

import "errors"

// Code 는 앱이 분기에 사용하는 에러 코드다.
type Code string

const (
	ValidationFailed        Code = "VALIDATION_FAILED"
	Unauthorized            Code = "UNAUTHORIZED"
	AuthInvalidIDToken      Code = "AUTH_INVALID_ID_TOKEN"
	AuthInvalidRefreshToken Code = "AUTH_INVALID_REFRESH_TOKEN"
	Forbidden               Code = "FORBIDDEN"
	CoupleRequired          Code = "COUPLE_REQUIRED"
	NotFound                Code = "NOT_FOUND"
	CoupleAlreadyConnected  Code = "COUPLE_ALREADY_CONNECTED"
	InviteInvalid           Code = "INVITE_INVALID"
	DateAlreadyInProgress   Code = "DATE_ALREADY_IN_PROGRESS"
	DateNotActive           Code = "DATE_NOT_ACTIVE"
	DateNotJoined           Code = "DATE_NOT_JOINED"
	AlreadySubmitted        Code = "ALREADY_SUBMITTED"
	FilmExhausted           Code = "FILM_EXHAUSTED"
	PhotoNotUploaded        Code = "PHOTO_NOT_UPLOADED"
	PhotoInvalid            Code = "PHOTO_INVALID"
	ReceiveNotAvailable     Code = "RECEIVE_NOT_AVAILABLE"
	Internal                Code = "INTERNAL_ERROR"
)

// Error 는 코드와 사람용 메시지를 가진 도메인 에러다. 메시지는 외부에 그대로 노출해도 안전해야 한다.
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// New 는 도메인 에러를 만든다.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// As 는 err 체인에서 도메인 에러를 찾는다.
func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

// Is 는 err 가 주어진 코드의 도메인 에러인지 확인한다.
func Is(err error, code Code) bool {
	e, ok := As(err)
	return ok && e.Code == code
}
