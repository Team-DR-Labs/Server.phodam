package postgres

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/config"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

var errRollback = errors.New("rollback")

// withTestTx 는 마이그레이션된 로컬 DB(make up)에 연결해 fn 을 트랜잭션 안에서 실행하고 항상 롤백한다.
// DB 에 연결할 수 없으면 테스트를 건너뛴다.
func withTestTx(t *testing.T, fn func(ctx context.Context)) {
	t.Helper()
	port, _ := strconv.Atoi(envOr("DB_PORT", "5432"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := Open(ctx, config.DBConfig{
		Host: envOr("DB_HOST", "localhost"), Port: port, User: envOr("DB_USER", "phodam"),
		Password: envOr("DB_PASSWORD", "phodam"), Name: envOr("DB_NAME", "phodam"),
		SSLMode: "disable", MaxOpenConns: 2, MaxIdleConns: 1, ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer Close(db)
	var n int64
	if err := db.Raw(`SELECT count(*) FROM themes`).Scan(&n).Error; err != nil || n == 0 {
		t.Skip("schema not migrated")
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		fn(context.WithValue(ctx, txKey{}, tx))
		return errRollback
	})
	if !errors.Is(err, errRollback) {
		t.Fatalf("transaction: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type fixture struct {
	users  *UserRepository
	dates  *DateRepository
	photos *PhotoRepository
	a, b   uuid.UUID
	couple uuid.UUID
}

func newFixture(t *testing.T, ctx context.Context) fixture {
	t.Helper()
	f := fixture{users: NewUserRepository(nil), dates: NewDateRepository(nil), photos: NewPhotoRepository(nil), a: uuid.New(), b: uuid.New(), couple: uuid.New()}
	now := time.Now().UTC()
	must(t, f.users.Create(ctx, user.New(f.a, "a", now)))
	must(t, f.users.Create(ctx, user.New(f.b, "b", now)))
	must(t, NewCoupleRepository(nil).Create(ctx, couple.Couple{ID: f.couple, UserAID: f.a, UserBID: f.b, Status: couple.StatusActive, ConnectedAt: now}))
	return f
}

func (f fixture) startDate(t *testing.T, ctx context.Context, at time.Time) dating.Date {
	t.Helper()
	themes, err := f.dates.ListThemes(ctx)
	must(t, err)
	topics, err := f.dates.ListTopics(ctx, themes[0].ID)
	must(t, err)
	d := dating.Start(dating.StartParams{ID: uuid.New(), CoupleID: f.couple, Theme: themes[0], Starter: f.a, Partner: f.b,
		StarterTopic: topics[0], PartnerTopic: topics[1], Now: at})
	must(t, f.dates.Create(ctx, d))
	return d
}

func (f fixture) addPhoto(t *testing.T, ctx context.Context, d dating.Date, owner uuid.UUID, st photo.Status) photo.Photo {
	t.Helper()
	p := photo.NewReserved(uuid.New(), d.ID, owner, d.StartedAt)
	p.Status = st
	must(t, f.photos.Create(ctx, p))
	return p
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_DateLifecycleQueries(t *testing.T) {
	withTestTx(t, func(ctx context.Context) {
		f := newFixture(t, ctx)
		past := time.Now().UTC().Add(-80 * time.Hour).Truncate(time.Microsecond)
		d := f.startDate(t, ctx, past)

		// 유니크 위반은 트랜잭션을 중단시키므로 savepoint(중첩 트랜잭션) 안에서 확인한다.
		err := conn(ctx, nil).Transaction(func(tx *gorm.DB) error {
			return f.dates.Create(context.WithValue(ctx, txKey{}, tx), d)
		})
		if !errors.Is(err, out.ErrDuplicate) {
			t.Fatalf("second in_progress date must be duplicate, got %v", err)
		}
		got, err := f.dates.GetForUpdate(ctx, d.ID)
		must(t, err)
		if len(got.Participants) != 2 || got.Theme.Title == "" || !got.StartedAt.Equal(past) {
			t.Fatalf("unexpected date: %+v", got)
		}

		rep := f.addPhoto(t, ctx, d, f.a, photo.StatusArchived)
		joined, _, _ := got.Join(f.b, past)
		submitted, _, err := joined.Submit(f.a, rep.ID, nil, past.Add(time.Hour))
		must(t, err)
		me, _ := submitted.Participant(f.a)
		must(t, f.dates.UpdateParticipant(ctx, d.ID, me))
		bUploaded := f.addPhoto(t, ctx, d, f.b, photo.StatusUploaded)
		aUploaded := f.addPhoto(t, ctx, d, f.a, photo.StatusUploaded)

		now := time.Now().UTC()
		expired, err := f.dates.ExpireDue(ctx, now, 1000)
		must(t, err)
		if !containsID(expired, d.ID) {
			t.Fatal("past-deadline date must expire")
		}
		left, err := f.photos.ListExpiredLeftovers(ctx, 1000)
		must(t, err)
		if !containsPhoto(left, bUploaded.ID) || containsPhoto(left, aUploaded.ID) || containsPhoto(left, rep.ID) {
			t.Fatal("only unsubmitted participant's live photos are leftovers")
		}
		overdue, err := f.photos.ListReceiveOverdue(ctx, now.Add(8*24*time.Hour), 1000)
		must(t, err)
		if !containsPhoto(overdue, aUploaded.ID) || containsPhoto(overdue, rep.ID) {
			t.Fatal("receive overdue targets uploaded non-representative photos only")
		}
		archived, err := f.photos.ListArchivedWithTemp(ctx, 1000)
		must(t, err)
		if !containsPhoto(archived, rep.ID) {
			t.Fatal("archived photo without temp purge must be listed")
		}
		changed, err := f.photos.MarkDeleted(ctx, bUploaded.ID, now, photo.StatusUploaded)
		must(t, err)
		again, err := f.photos.MarkDeleted(ctx, bUploaded.ID, now, photo.StatusUploaded)
		must(t, err)
		if !changed || again {
			t.Fatalf("conditional delete: first=%v second=%v", changed, again)
		}

		page, err := f.dates.ListSubmittedByUser(ctx, f.a, nil, 10)
		must(t, err)
		if len(page) != 1 || page[0].ID != d.ID {
			t.Fatalf("diary list: %+v", page)
		}
		next, err := f.dates.ListSubmittedByUser(ctx, f.a, &diary.Cursor{StartedAt: page[0].StartedAt, DateID: page[0].ID}, 10)
		must(t, err)
		if len(next) != 0 {
			t.Fatal("cursor must exclude the last item")
		}
	})
}

func TestIntegration_ReminderAndFilm(t *testing.T) {
	withTestTx(t, func(ctx context.Context) {
		f := newFixture(t, ctx)
		now := time.Now().UTC()
		d := f.startDate(t, ctx, now.Add(-dating.SubmitWindow+5*time.Hour))

		rs, err := f.dates.ClaimReminders(ctx, now, 1000)
		must(t, err)
		var mine *out.Reminder
		for i := range rs {
			if rs[i].DateID == d.ID {
				mine = &rs[i]
			}
		}
		if mine == nil || len(mine.UserIDs) != 2 {
			t.Fatalf("reminder for both unsubmitted participants: %+v", mine)
		}
		again, err := f.dates.ClaimReminders(ctx, now, 1000)
		must(t, err)
		for _, r := range again {
			if r.DateID == d.ID {
				t.Fatal("reminder must be claimed once")
			}
		}

		must(t, f.users.LockForUpdate(ctx, f.b, f.a))
		bal, err := f.users.AddFilm(ctx, f.a, -user.SignupFilm)
		must(t, err)
		if _, ok, err := f.users.DecrementFilm(ctx, f.a); bal != 0 || ok || err != nil {
			t.Fatalf("decrement at zero must not apply: bal=%d ok=%v err=%v", bal, ok, err)
		}
		if _, err := f.users.AddFilm(ctx, uuid.New(), 1); !errors.Is(err, out.ErrNotFound) {
			t.Fatal("unknown user must be ErrNotFound")
		}
	})
}

func TestIntegration_RefreshAndDevices(t *testing.T) {
	withTestTx(t, func(ctx context.Context) {
		f := newFixture(t, ctx)
		rt := NewRefreshTokenRepository(nil)
		now := time.Now().UTC()
		hash := uuid.NewString() + uuid.NewString()
		must(t, rt.Create(ctx, out.RefreshToken{ID: uuid.New(), UserID: f.a, TokenHash: hash[:64], ExpiresAt: now.Add(time.Hour), CreatedAt: now}))
		id, err := rt.Consume(ctx, hash[:64], now)
		if err != nil || id != f.a {
			t.Fatalf("consume: %v %v", id, err)
		}
		if _, err := rt.Consume(ctx, hash[:64], now); !errors.Is(err, out.ErrNotFound) {
			t.Fatal("consumed token must not be reusable")
		}

		dev := NewDeviceRepository(nil)
		token := "it-" + uuid.NewString()
		must(t, dev.Upsert(ctx, user.Device{UserID: f.a, Token: token, Platform: user.PlatformIOS}))
		must(t, dev.Upsert(ctx, user.Device{UserID: f.b, Token: token, Platform: user.PlatformAndroid}))
		tokens, err := dev.ListTokens(ctx, f.b)
		must(t, err)
		if len(tokens) != 1 || tokens[0] != token {
			t.Fatalf("token owner should move: %v", tokens)
		}
		must(t, dev.DeleteTokens(ctx, token))
	})
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func containsPhoto(ps []photo.Photo, id uuid.UUID) bool {
	for _, p := range ps {
		if p.ID == id {
			return true
		}
	}
	return false
}
