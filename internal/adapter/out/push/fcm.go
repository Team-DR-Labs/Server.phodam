package push

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

// fcmBatchLimit 은 멀티캐스트 한 번에 보낼 수 있는 최대 토큰 수다.
const fcmBatchLimit = 500

// FCMSender 는 FCM HTTP v1 out.PushSender 구현체다.
type FCMSender struct {
	client *messaging.Client
}

var _ out.PushSender = (*FCMSender)(nil)

// NewFCMSender 는 서비스 계정 JSON 파일로 FCM 클라이언트를 만든다.
func NewFCMSender(ctx context.Context, credentialsFile string) (*FCMSender, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithAuthCredentialsFile(option.ServiceAccount, credentialsFile))
	if err != nil {
		return nil, fmt.Errorf("init firebase app: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("init fcm client: %w", err)
	}
	return &FCMSender{client: client}, nil
}

// Send 는 알림을 보내고 UNREGISTERED 응답을 받은 토큰을 돌려준다.
func (s *FCMSender) Send(ctx context.Context, tokens []string, msg push.Message) ([]string, error) {
	var invalid []string
	for start := 0; start < len(tokens); start += fcmBatchLimit {
		batch := tokens[start:min(start+fcmBatchLimit, len(tokens))]
		res, err := s.client.SendEachForMulticast(ctx, multicast(batch, msg))
		if err != nil {
			return invalid, fmt.Errorf("fcm send: %w", err)
		}
		invalid = append(invalid, unregistered(batch, res)...)
	}
	return invalid, nil
}

func multicast(tokens []string, msg push.Message) *messaging.MulticastMessage {
	return &messaging.MulticastMessage{
		Tokens:       tokens,
		Data:         msg.Data(),
		Notification: &messaging.Notification{Title: msg.Title, Body: msg.Body},
	}
}

func unregistered(tokens []string, res *messaging.BatchResponse) []string {
	var invalid []string
	for i, r := range res.Responses {
		if r != nil && !r.Success && messaging.IsUnregistered(r.Error) {
			invalid = append(invalid, tokens[i])
		}
	}
	return invalid
}
