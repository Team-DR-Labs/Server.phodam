package http

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

type fakeDiary struct {
	page     diary.Page
	detail   diary.Detail
	err      error
	gotLimit int
	gotCur   string
}

func (f *fakeDiary) List(_ context.Context, _ uuid.UUID, cursor string, limit int) (diary.Page, error) {
	f.gotCur, f.gotLimit = cursor, limit
	return f.page, f.err
}
func (f *fakeDiary) Get(context.Context, uuid.UUID, uuid.UUID) (diary.Detail, error) {
	return f.detail, f.err
}

func TestDiaryHandler_List(t *testing.T) {
	uc := &fakeDiary{page: diary.Page{Items: []diary.ListItem{{
		DateID: uuid.New(), LocalDate: "2026-10-05", Theme: dating.Theme{ID: uuid.New(), Title: "온기"},
		Visibility: diary.VisibilityWaiting, StartedAt: testTime, Thumbnail: diary.URL{URL: "https://s/thumb", ExpiresAt: testTime},
	}}}}
	r := newV1Router(NewDiaryHandler(uc, &fakeAuth{}, discard))

	rec := do(t, r, call{method: http.MethodGet, path: "/v1/diaries?limit=5&cursor=abc", token: "good"})

	if rec.Code != http.StatusOK || uc.gotLimit != 5 || uc.gotCur != "abc" {
		t.Fatalf("status=%d limit=%d cursor=%q", rec.Code, uc.gotLimit, uc.gotCur)
	}
	body := rec.Body.String()
	for _, want := range []string{`"local_date":"2026-10-05"`, `"visibility":"waiting"`, `"thumbnail_url":"https://s/thumb"`, `"next_cursor":null`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s missing %s", body, want)
		}
	}
	wantError(t, do(t, r, call{method: http.MethodGet, path: "/v1/diaries?limit=x", token: "good"}), 400, apperr.ValidationFailed)
	wantError(t, do(t, r, call{method: http.MethodGet, path: "/v1/diaries?limit=0", token: "good"}), 400, apperr.ValidationFailed)
}

func TestDiaryHandler_Detail(t *testing.T) {
	uc := &fakeDiary{detail: diary.Detail{DateID: uuid.New(), Visibility: diary.VisibilityShared, Entries: []diary.Entry{
		{Author: user.Profile{ID: testUser, Nickname: "나"}, IsMe: true, Topic: dating.Topic{ID: uuid.New(), Title: "t"}, SubmittedAt: testTime},
	}}}
	r := newV1Router(NewDiaryHandler(uc, &fakeAuth{}, discard))
	rec := do(t, r, call{method: http.MethodGet, path: "/v1/diaries/" + uuid.NewString(), token: "good"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"is_me":true`) || !strings.Contains(rec.Body.String(), `"caption":null`) {
		t.Fatalf("detail: %d %s", rec.Code, rec.Body.String())
	}

	notFound := newV1Router(NewDiaryHandler(&fakeDiary{err: apperr.New(apperr.NotFound, "x")}, &fakeAuth{}, discard))
	wantError(t, do(t, notFound, call{method: http.MethodGet, path: "/v1/diaries/" + uuid.NewString(), token: "good"}), 404, apperr.NotFound)
}

func TestPhotoHandler_ReceiveErrors(t *testing.T) {
	r := dateRouter(&fakeDates{}, fakePhotos{err: apperr.New(apperr.ReceiveNotAvailable, "x")})
	wantError(t, do(t, r, call{method: http.MethodGet, path: "/v1/dates/" + uuid.NewString() + "/receivable", token: "good"}), 409, apperr.ReceiveNotAvailable)
	wantError(t, do(t, r, call{method: http.MethodPost, path: "/v1/photos/" + uuid.NewString() + "/received", token: "good"}), 409, apperr.ReceiveNotAvailable)

	ok := dateRouter(&fakeDates{}, fakePhotos{})
	rec := do(t, ok, call{method: http.MethodGet, path: "/v1/dates/" + uuid.NewString() + "/receivable", token: "good"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("receivable: %d %s", rec.Code, rec.Body.String())
	}
}
