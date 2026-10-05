// Package health 는 서비스 상태 점검 도메인 모델을 정의한다.
// 외부 프레임워크(gin, gorm)에 의존하지 않는 순수 Go 패키지다.
package health

// Status 는 서비스 또는 구성 요소의 상태다.
type Status string

const (
	StatusUp   Status = "up"
	StatusDown Status = "down"
)

// Component 는 개별 의존성(DB 등)의 점검 결과다.
type Component struct {
	Name   string
	Status Status
	Error  string
}

// Report 는 전체 상태 점검 결과다.
type Report struct {
	Status     Status
	Components []Component
}

// NewReport 는 구성 요소 결과를 모아 전체 상태를 계산한다.
// 하나라도 down 이면 전체가 down 이다.
func NewReport(components ...Component) Report {
	status := StatusUp
	for _, c := range components {
		if c.Status != StatusUp {
			status = StatusDown
			break
		}
	}
	return Report{Status: status, Components: append([]Component(nil), components...)}
}

// IsUp 은 전체 상태가 정상인지 반환한다.
func (r Report) IsUp() bool {
	return r.Status == StatusUp
}
