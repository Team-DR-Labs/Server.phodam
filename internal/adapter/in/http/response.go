package http

import "github.com/Team-DR-Labs/Server.phodam/internal/domain/health"

// HealthResponse 는 상태 점검 응답 본문이다.
type HealthResponse struct {
	Status     health.Status       `json:"status"`
	Components []ComponentResponse `json:"components,omitempty"`
}

// ComponentResponse 는 구성 요소별 상태다.
// 내부 에러 메시지는 외부에 노출하지 않고 서버 로그로만 남긴다.
type ComponentResponse struct {
	Name   string        `json:"name"`
	Status health.Status `json:"status"`
}

// ErrorResponse 는 공통 에러 응답 본문이다.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func toHealthResponse(r health.Report) HealthResponse {
	components := make([]ComponentResponse, 0, len(r.Components))
	for _, c := range r.Components {
		components = append(components, ComponentResponse{Name: c.Name, Status: c.Status})
	}
	return HealthResponse{Status: r.Status, Components: components}
}
