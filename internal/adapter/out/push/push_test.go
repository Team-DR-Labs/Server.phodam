package push

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

func TestMulticastPayload(t *testing.T) {
	dateID := uuid.New()
	m := multicast([]string{"t1"}, push.NewMessage(push.TypeDateRevealed, dateID))
	if m.Data["type"] != "date_revealed" || m.Data["date_id"] != dateID.String() || m.Notification.Title == "" {
		t.Fatalf("unexpected payload: %+v", m)
	}
}

func TestUnregisteredIgnoresOtherFailures(t *testing.T) {
	res := &messaging.BatchResponse{Responses: []*messaging.SendResponse{
		{Success: true}, {Success: false, Error: errors.New("quota")},
	}}
	if got := unregistered([]string{"a", "b"}, res); len(got) != 0 {
		t.Fatalf("only UNREGISTERED tokens are invalid, got %v", got)
	}
}

func TestLogSender(t *testing.T) {
	invalid, err := NewLogSender(slog.New(slog.NewTextHandler(io.Discard, nil))).
		Send(context.Background(), []string{"t"}, push.NewMessage(push.TypeDeadlineSoon, uuid.New()))
	if err != nil || invalid != nil {
		t.Fatalf("log sender: %v %v", invalid, err)
	}
}
