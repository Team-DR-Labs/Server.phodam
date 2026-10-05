package http

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

type fakeDates struct {
	view      dating.View
	current   *dating.View
	err       error
	gotPhoto  uuid.UUID
	gotCaptin *string
}

func (f *fakeDates) Start(context.Context, uuid.UUID) (dating.View, error) { return f.view, f.err }
func (f *fakeDates) Current(context.Context, uuid.UUID) (*dating.View, error) {
	return f.current, f.err
}
func (f *fakeDates) Get(context.Context, uuid.UUID, uuid.UUID) (dating.View, error) {
	return f.view, f.err
}
func (f *fakeDates) Join(context.Context, uuid.UUID, uuid.UUID) (dating.View, error) {
	return f.view, f.err
}
func (f *fakeDates) Submit(_ context.Context, _, _, p uuid.UUID, c *string) (dating.View, error) {
	f.gotPhoto, f.gotCaptin = p, c
	return f.view, f.err
}

type fakePhotos struct {
	res in.ShotReservation
	err error
}

func (f fakePhotos) ReserveShot(context.Context, uuid.UUID, uuid.UUID) (in.ShotReservation, error) {
	return f.res, f.err
}
func (f fakePhotos) ReissueUploadURL(context.Context, uuid.UUID, uuid.UUID) (in.UploadTarget, error) {
	return f.res.Upload, f.err
}
func (f fakePhotos) CompleteUpload(context.Context, uuid.UUID, uuid.UUID) (photo.Photo, error) {
	return f.res.Photo, f.err
}
func (f fakePhotos) ListMine(context.Context, uuid.UUID, uuid.UUID) ([]in.PhotoWithURL, error) {
	return []in.PhotoWithURL{{Photo: f.res.Photo, URL: "https://s/get", URLExpiresAt: testTime}}, f.err
}
func (f fakePhotos) Receivable(context.Context, uuid.UUID, uuid.UUID) (in.Receivable, error) {
	return in.Receivable{ReceiveDeadlineAt: testTime}, f.err
}
func (f fakePhotos) AckReceived(context.Context, uuid.UUID, uuid.UUID) (photo.Photo, error) {
	return f.res.Photo, f.err
}

func sampleView() dating.View {
	topic := dating.Topic{ID: uuid.New(), Title: "따뜻한 색"}
	return dating.View{
		ID: uuid.New(), Status: dating.StatusInProgress, Theme: dating.Theme{ID: uuid.New(), Title: "온기"},
		StartedByMe: true, StartedAt: testTime, DeadlineAt: testTime,
		Me:      dating.MyParticipation{Status: dating.ParticipantJoined, Topic: &topic, JoinedAt: &testTime},
		Partner: dating.PartnerParticipation{User: user.Profile{ID: uuid.New(), Nickname: "밥"}, Status: dating.ParticipantAssigned},
	}
}

func dateRouter(d *fakeDates, p fakePhotos) http.Handler {
	return newV1Router(NewDateHandler(d, p, &fakeAuth{}, discard), NewPhotoHandler(p, &fakeAuth{}, discard))
}

func TestDateHandler_StartAndCurrent(t *testing.T) {
	r := dateRouter(&fakeDates{view: sampleView()}, fakePhotos{})

	rec := do(t, r, call{method: http.MethodPost, path: "/v1/dates", token: "good"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("start status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"title":"따뜻한 색"`, `"partner":{"user":{`, `"topic":null`, `"revealed_at":null`, `"shot_count":0`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s missing %s", body, want)
		}
	}

	rec = do(t, r, call{method: http.MethodGet, path: "/v1/dates/current", token: "good"})
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("current without date: %d %q", rec.Code, rec.Body.String())
	}
}

func TestDateHandler_ErrorsAndPathValidation(t *testing.T) {
	tests := []struct {
		name   string
		dates  *fakeDates
		photos fakePhotos
		call   call
		status int
		code   apperr.Code
	}{
		{"bad uuid", &fakeDates{}, fakePhotos{}, call{method: http.MethodGet, path: "/v1/dates/not-a-uuid", token: "good"}, 404, apperr.NotFound},
		{"couple required", &fakeDates{err: apperr.New(apperr.CoupleRequired, "x")}, fakePhotos{}, call{method: http.MethodPost, path: "/v1/dates", token: "good"}, 403, apperr.CoupleRequired},
		{"in progress", &fakeDates{err: apperr.New(apperr.DateAlreadyInProgress, "x")}, fakePhotos{}, call{method: http.MethodPost, path: "/v1/dates", token: "good"}, 409, apperr.DateAlreadyInProgress},
		{"film exhausted", &fakeDates{}, fakePhotos{err: apperr.New(apperr.FilmExhausted, "x")}, call{method: http.MethodPost, path: "/v1/dates/" + uuid.NewString() + "/shots", token: "good"}, 409, apperr.FilmExhausted},
		{"photo invalid", &fakeDates{}, fakePhotos{err: apperr.New(apperr.PhotoInvalid, "x")}, call{method: http.MethodPost, path: "/v1/photos/" + uuid.NewString() + "/complete", token: "good"}, 400, apperr.PhotoInvalid},
		{"submit bad photo id", &fakeDates{}, fakePhotos{}, call{method: http.MethodPost, path: "/v1/dates/" + uuid.NewString() + "/submit", body: `{"photo_id":"x"}`, token: "good"}, 400, apperr.ValidationFailed},
		{"no token", &fakeDates{}, fakePhotos{}, call{method: http.MethodGet, path: "/v1/dates/current"}, 401, apperr.Unauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantError(t, do(t, dateRouter(tt.dates, tt.photos), tt.call), tt.status, tt.code)
		})
	}
}

func TestDateHandler_ShotResponse(t *testing.T) {
	p := photo.NewReserved(uuid.New(), uuid.New(), testUser, testTime)
	res := in.ShotReservation{Photo: p, FilmBalance: 23, Upload: in.UploadTarget{
		URL: "http://localhost:9000/put", Method: "PUT", Headers: map[string]string{"Content-Type": "image/jpeg"}, ExpiresAt: testTime,
	}}
	r := dateRouter(&fakeDates{}, fakePhotos{res: res})

	rec := do(t, r, call{method: http.MethodPost, path: "/v1/dates/" + uuid.NewString() + "/shots", token: "good"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"film_balance":23`, `"method":"PUT"`, `"Content-Type":"image/jpeg"`, `"status":"reserved"`, `"uploaded_at":null`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s missing %s", body, want)
		}
	}
	if strings.Contains(body, "temp/") {
		t.Fatal("object keys must not be exposed")
	}
}

func TestDateHandler_SubmitPassesCaption(t *testing.T) {
	d := &fakeDates{view: sampleView()}
	photoID := uuid.New()
	rec := do(t, dateRouter(d, fakePhotos{}), call{method: http.MethodPost, path: "/v1/dates/" + uuid.NewString() + "/submit",
		body: `{"photo_id":"` + photoID.String() + `","caption":"좋았다"}`, token: "good"})
	if rec.Code != http.StatusOK || d.gotPhoto != photoID || d.gotCaptin == nil || *d.gotCaptin != "좋았다" {
		t.Fatalf("submit: %d photo=%v caption=%v", rec.Code, d.gotPhoto, d.gotCaptin)
	}
}
