// Package push 는 푸시 발송 아웃바운드 어댑터다.
package push

import (
	"context"
	"log/slog"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

// LogSender 는 FCM 자격 증명이 없을 때 쓰는 out.PushSender 구현체다. 발송 대신 slog 로 출력한다.
type LogSender struct {
	logger *slog.Logger
}

var _ out.PushSender = (*LogSender)(nil)

// NewLogSender 는 LogSender 를 생성한다.
func NewLogSender(logger *slog.Logger) *LogSender { return &LogSender{logger: logger} }

// Send 는 알림 내용을 로그로 남긴다. 토큰 원문은 남기지 않는다.
func (s *LogSender) Send(ctx context.Context, tokens []string, msg push.Message) ([]string, error) {
	s.logger.InfoContext(ctx, "push (log sender)",
		slog.String("type", string(msg.Type)),
		slog.String("date_id", msg.DateID.String()),
		slog.String("title", msg.Title),
		slog.Int("devices", len(tokens)),
	)
	return nil, nil
}
