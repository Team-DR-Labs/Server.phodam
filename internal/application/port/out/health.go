// Package out 은 애플리케이션이 외부 인프라(DB 등)를 호출하는 아웃바운드 포트를 정의한다.
package out

import "context"

// DatabasePinger 는 데이터베이스 연결 상태를 확인한다.
type DatabasePinger interface {
	Ping(ctx context.Context) error
}
