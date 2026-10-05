package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

var (
	testUser  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testTime  = time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)
	discard   = slog.New(slog.NewTextHandler(io.Discard, nil))
	errSecret = errors.New("pq: secret table detail")
)

// fakeAuth 는 "good" 토큰만 testUser 로 인증한다.
type fakeAuth struct {
	login  in.AuthResult
	err    error
	gotDev string
}

func (f *fakeAuth) LoginApple(context.Context, string, *string) (in.AuthResult, error) {
	return f.login, f.err
}
func (f *fakeAuth) LoginGoogle(context.Context, string) (in.AuthResult, error) { return f.login, f.err }
func (f *fakeAuth) LoginDev(_ context.Context, id string, _ *string) (in.AuthResult, error) {
	f.gotDev = id
	return f.login, f.err
}
func (f *fakeAuth) Refresh(context.Context, string) (in.AuthResult, error) { return f.login, f.err }
func (f *fakeAuth) Logout(context.Context, uuid.UUID, string) error        { return f.err }
func (f *fakeAuth) Authenticate(_ context.Context, tok string) (uuid.UUID, error) {
	if tok == "good" {
		return testUser, nil
	}
	return uuid.Nil, apperr.New(apperr.Unauthorized, "invalid")
}

func newV1Router(handlers ...Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(discard, NewGroup("/v1", handlers...))
}

type call struct {
	method, path, body string
	token              string
	header             map[string]string
}

func do(t *testing.T, r http.Handler, c call) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if c.body != "" {
		body = bytes.NewBufferString(c.body)
	}
	req := httptest.NewRequest(c.method, c.path, body)
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for k, v := range c.header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func wantError(t *testing.T, rec *httptest.ResponseRecorder, status int, code apperr.Code) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	if got := decode[ErrorResponse](t, rec); got.Code != string(code) {
		t.Fatalf("code = %s, want %s", got.Code, code)
	}
}

func sampleProfile() user.Profile { return user.Profile{ID: testUser, Nickname: "앨리스"} }
