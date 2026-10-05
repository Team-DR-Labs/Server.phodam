package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
)

var ctx = context.Background()

func wantCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	if !apperr.Is(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func (h *harness) userByNick(nick string) uuid.UUID {
	for id, u := range h.st.users {
		if u.Nickname == nick {
			return id
		}
	}
	return uuid.Nil
}
