package out

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound 는 리포지토리에서 대상 행이 없을 때 반환한다.
	ErrNotFound = errors.New("not found")
	// ErrDuplicate 는 유니크 제약 위반이다.
	ErrDuplicate = errors.New("duplicate")
	// ErrInvalidIDToken 은 외부 ID 토큰의 서명·클레임 검증 실패다. 설정·네트워크 오류와 구분한다.
	ErrInvalidIDToken = errors.New("invalid id token")
	// ErrObjectNotFound 는 오브젝트 스토리지에 객체가 없을 때 반환한다.
	ErrObjectNotFound = errors.New("object not found")
)

// Clock 은 현재 시각(UTC)을 제공한다. 시간 의존 로직은 모두 이 포트를 쓴다.
type Clock interface {
	Now() time.Time
}

// TxManager 는 Unit of Work 다. fn 에 넘긴 ctx 를 쓰는 리포지토리 호출은 같은 트랜잭션에 묶인다.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
