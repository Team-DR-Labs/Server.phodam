package http

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
)

type fakeAdmin struct{ err error }

func (f fakeAdmin) GrantFilm(_ context.Context, _ uuid.UUID, shots int, _ *string) (int, error) {
	return shots + 1, f.err
}

func TestAdminHandler(t *testing.T) {
	r := newV1Router(NewAdminHandler(fakeAdmin{}, "admin-secret", discard))
	path := "/v1/admin/users/" + uuid.NewString() + "/film-grants"

	rec := do(t, r, call{method: http.MethodPost, path: path, body: `{"shots":24}`, header: map[string]string{"X-Admin-Key": "admin-secret"}})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"film_balance":25`) {
		t.Fatalf("grant: %d %s", rec.Code, rec.Body.String())
	}
	wantError(t, do(t, r, call{method: http.MethodPost, path: path, body: `{"shots":24}`}), 403, apperr.Forbidden)
	wantError(t, do(t, r, call{method: http.MethodPost, path: path, body: `{"shots":24}`, header: map[string]string{"X-Admin-Key": "wrong"}}), 403, apperr.Forbidden)
	wantError(t, do(t, r, call{method: http.MethodPost, path: path, body: `{}`, header: map[string]string{"X-Admin-Key": "admin-secret"}}), 400, apperr.ValidationFailed)

	missing := newV1Router(NewAdminHandler(fakeAdmin{err: apperr.New(apperr.NotFound, "x")}, "admin-secret", discard))
	wantError(t, do(t, missing, call{method: http.MethodPost, path: path, body: `{"shots":1}`, header: map[string]string{"X-Admin-Key": "admin-secret"}}), 404, apperr.NotFound)
}
