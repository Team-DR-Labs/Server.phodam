// Package clock 은 시스템 시계 아웃바운드 어댑터다.
package clock

import (
	"time"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
)

// System 은 out.Clock 구현체다. DB(timestamptz) 정밀도에 맞춰 마이크로초로 자른 UTC 를 반환한다.
type System struct{}

var _ out.Clock = System{}

// Now 는 현재 UTC 시각이다.
func (System) Now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
