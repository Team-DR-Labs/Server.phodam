package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// memStore 는 모든 리포지토리 포트를 메모리로 흉내 낸다.
type memStore struct {
	mu         sync.Mutex
	users      map[uuid.UUID]user.User
	identities map[string]uuid.UUID
	ledger     []user.LedgerEntry
	refresh    map[string]*refreshRow
	devices    map[string]user.Device
	couples    []couple.Couple
	invites    map[string]couple.Invite
	themes     []dating.Theme
	topics     map[uuid.UUID][]dating.Topic
	dates      map[uuid.UUID]dating.Date
	photos     map[uuid.UUID]photo.Photo
}

type refreshRow struct {
	token   out.RefreshToken
	revoked bool
}

func newMemStore() *memStore {
	return &memStore{
		users: map[uuid.UUID]user.User{}, identities: map[string]uuid.UUID{},
		refresh: map[string]*refreshRow{}, devices: map[string]user.Device{},
		invites: map[string]couple.Invite{}, topics: map[uuid.UUID][]dating.Topic{},
		dates: map[uuid.UUID]dating.Date{}, photos: map[uuid.UUID]photo.Photo{},
	}
}

func (s *memStore) seedThemes(n, topicsPer int) {
	for i := 0; i < n; i++ {
		th := dating.Theme{ID: uuid.New(), Title: fmt.Sprintf("theme-%d", i)}
		s.themes = append(s.themes, th)
		for j := 0; j < topicsPer; j++ {
			s.topics[th.ID] = append(s.topics[th.ID], dating.Topic{ID: uuid.New(), Title: fmt.Sprintf("topic-%d-%d", i, j)})
		}
	}
}

func (s *memStore) ledgerSum(userID uuid.UUID) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum := 0
	for _, e := range s.ledger {
		if e.UserID == userID {
			sum += e.Delta
		}
	}
	return sum
}

// ---- tx / clock ----

type fakeTx struct{}

func (fakeTx) WithinTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// ---- users ----

type fakeUsers struct{ s *memStore }

func (f fakeUsers) Create(_ context.Context, u user.User) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.users[u.ID] = u
	return nil
}

func (f fakeUsers) Get(_ context.Context, id uuid.UUID) (user.User, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	u, ok := f.s.users[id]
	if !ok {
		return user.User{}, out.ErrNotFound
	}
	return u, nil
}

func (f fakeUsers) LockForUpdate(context.Context, ...uuid.UUID) error { return nil }

func (f fakeUsers) UpdateNickname(_ context.Context, id uuid.UUID, n string) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	u := f.s.users[id]
	u.Nickname = n
	f.s.users[id] = u
	return nil
}

func (f fakeUsers) DecrementFilm(_ context.Context, id uuid.UUID) (int, bool, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	u := f.s.users[id]
	if u.FilmBalance <= 0 {
		return 0, false, nil
	}
	u.FilmBalance--
	f.s.users[id] = u
	return u.FilmBalance, true, nil
}

func (f fakeUsers) AddFilm(_ context.Context, id uuid.UUID, n int) (int, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	u, ok := f.s.users[id]
	if !ok {
		return 0, out.ErrNotFound
	}
	u.FilmBalance += n
	f.s.users[id] = u
	return u.FilmBalance, nil
}

type fakeIdentities struct{ s *memStore }

func identityKey(p user.Provider, sub string) string { return string(p) + "|" + sub }

func (f fakeIdentities) FindUserID(_ context.Context, p user.Provider, sub string) (uuid.UUID, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	id, ok := f.s.identities[identityKey(p, sub)]
	if !ok {
		return uuid.Nil, out.ErrNotFound
	}
	return id, nil
}

func (f fakeIdentities) Create(_ context.Context, i user.Identity) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	k := identityKey(i.Provider, i.Subject)
	if _, ok := f.s.identities[k]; ok {
		return out.ErrDuplicate
	}
	f.s.identities[k] = i.UserID
	return nil
}

type fakeLedger struct{ s *memStore }

func (f fakeLedger) Append(_ context.Context, e user.LedgerEntry) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.ledger = append(f.s.ledger, e)
	return nil
}

type fakeRefresh struct{ s *memStore }

func (f fakeRefresh) Create(_ context.Context, t out.RefreshToken) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.refresh[t.TokenHash] = &refreshRow{token: t}
	return nil
}

func (f fakeRefresh) Consume(_ context.Context, hash string, now time.Time) (uuid.UUID, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	r, ok := f.s.refresh[hash]
	if !ok || r.revoked || !now.Before(r.token.ExpiresAt) {
		return uuid.Nil, out.ErrNotFound
	}
	r.revoked = true
	return r.token.UserID, nil
}

func (f fakeRefresh) Revoke(_ context.Context, userID uuid.UUID, hash string, _ time.Time) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	if r, ok := f.s.refresh[hash]; ok && r.token.UserID == userID {
		r.revoked = true
	}
	return nil
}

type fakeDevices struct{ s *memStore }

func (f fakeDevices) Upsert(_ context.Context, d user.Device) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.devices[d.Token] = d
	return nil
}

func (f fakeDevices) ListTokens(_ context.Context, ids ...uuid.UUID) ([]string, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var res []string
	for tok, d := range f.s.devices {
		if slices.Contains(ids, d.UserID) {
			res = append(res, tok)
		}
	}
	sort.Strings(res)
	return res, nil
}

func (f fakeDevices) DeleteTokens(_ context.Context, tokens ...string) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for _, t := range tokens {
		delete(f.s.devices, t)
	}
	return nil
}

// ---- couples ----

type fakeCouples struct{ s *memStore }

func (f fakeCouples) FindActiveByUser(_ context.Context, id uuid.UUID) (couple.Couple, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for _, c := range f.s.couples {
		if c.UserAID == id || c.UserBID == id {
			return c, nil
		}
	}
	return couple.Couple{}, out.ErrNotFound
}

func (f fakeCouples) Create(_ context.Context, c couple.Couple) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.couples = append(f.s.couples, c)
	return nil
}

type fakeInvites struct{ s *memStore }

func (f fakeInvites) RevokeUnused(_ context.Context, creator uuid.UUID, now time.Time) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for k, inv := range f.s.invites {
		if inv.CreatorID == creator && inv.UsedAt == nil && inv.RevokedAt == nil {
			inv.RevokedAt = &now
			f.s.invites[k] = inv
		}
	}
	return nil
}

func (f fakeInvites) Create(_ context.Context, inv couple.Invite) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	if _, ok := f.s.invites[inv.Code]; ok {
		return out.ErrDuplicate
	}
	f.s.invites[inv.Code] = inv
	return nil
}

func (f fakeInvites) FindByCodeForUpdate(_ context.Context, code string) (couple.Invite, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	inv, ok := f.s.invites[code]
	if !ok {
		return couple.Invite{}, out.ErrNotFound
	}
	return inv, nil
}

func (f fakeInvites) MarkUsed(_ context.Context, id, by uuid.UUID, now time.Time) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for k, inv := range f.s.invites {
		if inv.ID == id {
			inv.UsedAt, inv.UsedBy = &now, &by
			f.s.invites[k] = inv
		}
	}
	return nil
}

// ---- themes / dates ----

type fakeThemes struct{ s *memStore }

func (f fakeThemes) ListThemes(context.Context) ([]dating.Theme, error) {
	return append([]dating.Theme(nil), f.s.themes...), nil
}

func (f fakeThemes) ListTopics(_ context.Context, id uuid.UUID) ([]dating.Topic, error) {
	return append([]dating.Topic(nil), f.s.topics[id]...), nil
}

type fakeDates struct{ s *memStore }

func (f fakeDates) Create(_ context.Context, d dating.Date) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for _, cur := range f.s.dates {
		if cur.CoupleID == d.CoupleID && cur.Status == dating.StatusInProgress {
			return out.ErrDuplicate
		}
	}
	f.s.dates[d.ID] = d
	return nil
}

func (f fakeDates) Get(_ context.Context, id uuid.UUID) (dating.Date, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	d, ok := f.s.dates[id]
	if !ok {
		return dating.Date{}, out.ErrNotFound
	}
	return d, nil
}

func (f fakeDates) GetForUpdate(ctx context.Context, id uuid.UUID) (dating.Date, error) {
	return f.Get(ctx, id)
}

func (f fakeDates) FindInProgressByCouple(_ context.Context, coupleID uuid.UUID) (dating.Date, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	for _, d := range f.s.dates {
		if d.CoupleID == coupleID && d.Status == dating.StatusInProgress {
			return d, nil
		}
	}
	return dating.Date{}, out.ErrNotFound
}

func (f fakeDates) ThemeHistory(_ context.Context, coupleID uuid.UUID) (out.ThemeHistory, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	h := out.ThemeHistory{Used: map[uuid.UUID]bool{}}
	var lastAt time.Time
	for _, d := range f.s.dates {
		if d.CoupleID != coupleID {
			continue
		}
		h.Used[d.Theme.ID] = true
		if h.Last == nil || d.StartedAt.After(lastAt) {
			id := d.Theme.ID
			h.Last, lastAt = &id, d.StartedAt
		}
	}
	return h, nil
}

func (f fakeDates) UpdateParticipant(_ context.Context, dateID uuid.UUID, p dating.Participant) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	d := f.s.dates[dateID]
	parts := make([]dating.Participant, len(d.Participants))
	for i, cur := range d.Participants {
		parts[i] = cur
		if cur.UserID == p.UserID {
			parts[i] = p
		}
	}
	d.Participants = parts
	f.s.dates[dateID] = d
	return nil
}

func (f fakeDates) MarkRevealed(_ context.Context, id uuid.UUID, at time.Time) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	d := f.s.dates[id]
	d.Status, d.RevealedAt = dating.StatusRevealed, &at
	f.s.dates[id] = d
	return nil
}

func (f fakeDates) Expire(_ context.Context, id uuid.UUID, now time.Time) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	d := f.s.dates[id]
	if d.Status == dating.StatusInProgress && !now.Before(d.DeadlineAt) {
		d.Status, d.ExpiredAt = dating.StatusExpired, &now
		f.s.dates[id] = d
	}
	return nil
}

func (f fakeDates) ListSubmittedByUser(_ context.Context, userID uuid.UUID, after *diary.Cursor, limit int) ([]dating.Date, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var res []dating.Date
	for _, d := range f.s.dates {
		me, ok := d.Participant(userID)
		if !ok || !me.HasSubmitted() {
			continue
		}
		if after != nil && !beforeCursor(d, *after) {
			continue
		}
		res = append(res, d)
	}
	sort.Slice(res, func(i, j int) bool {
		if !res[i].StartedAt.Equal(res[j].StartedAt) {
			return res[i].StartedAt.After(res[j].StartedAt)
		}
		return res[i].ID.String() > res[j].ID.String()
	})
	if len(res) > limit {
		res = res[:limit]
	}
	return res, nil
}

func beforeCursor(d dating.Date, c diary.Cursor) bool {
	if d.StartedAt.Equal(c.StartedAt) {
		return d.ID.String() < c.DateID.String()
	}
	return d.StartedAt.Before(c.StartedAt)
}

func (f fakeDates) ExpireDue(_ context.Context, now time.Time, limit int) ([]uuid.UUID, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var ids []uuid.UUID
	for id, d := range f.s.dates {
		if len(ids) >= limit {
			break
		}
		if d.Status == dating.StatusInProgress && !now.Before(d.DeadlineAt) {
			d.Status, d.ExpiredAt = dating.StatusExpired, &now
			f.s.dates[id] = d
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (f fakeDates) ClaimReminders(_ context.Context, now time.Time, limit int) ([]out.Reminder, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var res []out.Reminder
	for id, d := range f.s.dates {
		if len(res) >= limit {
			break
		}
		due := !now.Before(d.DeadlineAt.Add(-dating.ReminderLead)) && now.Before(d.DeadlineAt)
		if d.Status != dating.StatusInProgress || d.RemindedAt != nil || !due {
			continue
		}
		d.RemindedAt = &now
		f.s.dates[id] = d
		r := out.Reminder{DateID: id}
		for _, p := range d.Participants {
			if !p.HasSubmitted() {
				r.UserIDs = append(r.UserIDs, p.UserID)
			}
		}
		res = append(res, r)
	}
	return res, nil
}

// ---- photos ----

type fakePhotos struct{ s *memStore }

func (f fakePhotos) Create(_ context.Context, p photo.Photo) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.photos[p.ID] = p
	return nil
}

func (f fakePhotos) Get(_ context.Context, id uuid.UUID) (photo.Photo, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	p, ok := f.s.photos[id]
	if !ok {
		return photo.Photo{}, out.ErrNotFound
	}
	return p, nil
}

func (f fakePhotos) GetForUpdate(ctx context.Context, id uuid.UUID) (photo.Photo, error) {
	return f.Get(ctx, id)
}

func (f fakePhotos) Update(_ context.Context, p photo.Photo) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.photos[p.ID] = p
	return nil
}

func (f fakePhotos) ListByOwner(_ context.Context, dateID, owner uuid.UUID, statuses ...photo.Status) ([]photo.Photo, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var res []photo.Photo
	for _, p := range f.s.photos {
		if p.DateID == dateID && p.OwnerID == owner && slices.Contains(statuses, p.Status) {
			res = append(res, p)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].CreatedAt.Before(res[j].CreatedAt) })
	return res, nil
}

func (f fakePhotos) CountByOwner(_ context.Context, dateID, owner uuid.UUID) (int, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	n := 0
	for _, p := range f.s.photos {
		if p.DateID == dateID && p.OwnerID == owner {
			n++
		}
	}
	return n, nil
}

func (f fakePhotos) DeleteReserved(_ context.Context, dateID, owner uuid.UUID, now time.Time) ([]photo.Photo, error) {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var res []photo.Photo
	for id, p := range f.s.photos {
		if p.DateID == dateID && p.OwnerID == owner && p.Status == photo.StatusReserved {
			p.Status, p.DeletedAt = photo.StatusDeleted, &now
			f.s.photos[id] = p
			res = append(res, p)
		}
	}
	return res, nil
}

func (f fakePhotos) MarkTempPurged(_ context.Context, id uuid.UUID, now time.Time) error {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	p := f.s.photos[id]
	p.TempPurgedAt = &now
	f.s.photos[id] = p
	return nil
}

func (f fakePhotos) selectPhotos(limit int, keep func(photo.Photo, dating.Date) bool) []photo.Photo {
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	var res []photo.Photo
	for _, p := range f.s.photos {
		if len(res) >= limit {
			break
		}
		if keep(p, f.s.dates[p.DateID]) {
			res = append(res, p)
		}
	}
	return res
}

func (f fakePhotos) LockExpiredLeftovers(_ context.Context, limit int) ([]photo.Photo, error) {
	return f.selectPhotos(limit, func(p photo.Photo, d dating.Date) bool {
		me, _ := d.Participant(p.OwnerID)
		live := p.Status == photo.StatusReserved || p.Status == photo.StatusUploaded
		return d.Status == dating.StatusExpired && !me.HasSubmitted() && live
	}), nil
}

func (f fakePhotos) LockReceiveOverdue(_ context.Context, now time.Time, limit int) ([]photo.Photo, error) {
	return f.selectPhotos(limit, func(p photo.Photo, d dating.Date) bool {
		me, _ := d.Participant(p.OwnerID)
		overdue := me.ReceiveDeadlineAt != nil && !now.Before(*me.ReceiveDeadlineAt)
		return p.Status == photo.StatusUploaded && !p.IsRepresentative && overdue
	}), nil
}

func (f fakePhotos) LockArchivedWithTemp(_ context.Context, limit int) ([]photo.Photo, error) {
	return f.selectPhotos(limit, func(p photo.Photo, _ dating.Date) bool {
		return p.Status == photo.StatusArchived && p.TempPurgedAt == nil
	}), nil
}

// ---- external ----

type fakeStorage struct {
	mu         sync.Mutex
	objects    map[string]int64
	failRemove bool
}

func newFakeStorage() *fakeStorage { return &fakeStorage{objects: map[string]int64{}} }

func objKey(b out.Bucket, key string) string { return string(b) + "/" + key }

func (f *fakeStorage) put(b out.Bucket, key string, size int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[objKey(b, key)] = size
}

func (f *fakeStorage) has(b out.Bucket, key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[objKey(b, key)]
	return ok
}

func (f *fakeStorage) PresignPut(_ context.Context, b out.Bucket, key, _ string, _ time.Duration) (string, error) {
	return "https://storage.test/" + objKey(b, key) + "?put", nil
}

func (f *fakeStorage) PresignGet(_ context.Context, b out.Bucket, key string, _ time.Duration) (string, error) {
	return "https://storage.test/" + objKey(b, key) + "?get", nil
}

func (f *fakeStorage) Stat(_ context.Context, b out.Bucket, key string) (out.ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	size, ok := f.objects[objKey(b, key)]
	if !ok {
		return out.ObjectInfo{}, out.ErrObjectNotFound
	}
	return out.ObjectInfo{Size: size}, nil
}

func (f *fakeStorage) Copy(_ context.Context, sb out.Bucket, sk string, db out.Bucket, dk string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	size, ok := f.objects[objKey(sb, sk)]
	if !ok {
		return errors.New("source missing")
	}
	f.objects[objKey(db, dk)] = size
	return nil
}

func (f *fakeStorage) Remove(_ context.Context, b out.Bucket, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failRemove {
		return errors.New("storage down")
	}
	delete(f.objects, objKey(b, key))
	return nil
}

type sentPush struct {
	tokens []string
	msg    push.Message
}

type fakePush struct {
	mu   sync.Mutex
	sent []sentPush
}

func (f *fakePush) Send(_ context.Context, tokens []string, msg push.Message) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentPush{tokens: tokens, msg: msg})
	return nil, nil
}

// typesTo 는 token 으로 보낸 푸시 종류를 순서대로 반환한다.
func (f *fakePush) typesTo(token string) []push.Type {
	f.mu.Lock()
	defer f.mu.Unlock()
	var res []push.Type
	for _, s := range f.sent {
		if slices.Contains(s.tokens, token) {
			res = append(res, s.msg.Type)
		}
	}
	return res
}

type fakeVerifier struct{ claims map[string]out.IDTokenClaims }

func (f fakeVerifier) Verify(_ context.Context, _ user.Provider, token string) (out.IDTokenClaims, error) {
	c, ok := f.claims[token]
	if !ok {
		return out.IDTokenClaims{}, errors.New("bad token")
	}
	return c, nil
}

type fakeAccess struct{}

func (fakeAccess) Issue(id uuid.UUID, now time.Time) (string, time.Time, error) {
	return "access-" + id.String(), now.Add(time.Hour), nil
}

func (fakeAccess) Parse(token string, _ time.Time) (uuid.UUID, error) {
	return uuid.Parse(strings.TrimPrefix(token, "access-"))
}

// ---- harness ----

type harness struct {
	st      *memStore
	clock   *fakeClock
	storage *fakeStorage
	push    *fakePush
	deps    Deps
}

var baseTime = time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC)

func newHarness(t *testing.T) *harness {
	t.Helper()
	st := newMemStore()
	st.seedThemes(3, 3)
	h := &harness{st: st, clock: &fakeClock{now: baseTime}, storage: newFakeStorage(), push: &fakePush{}}
	h.deps = Deps{
		Clock: h.clock, Tx: fakeTx{},
		Users: fakeUsers{st}, Identities: fakeIdentities{st}, Ledger: fakeLedger{st},
		Refresh: fakeRefresh{st}, Devices: fakeDevices{st},
		Couples: fakeCouples{st}, Invites: fakeInvites{st},
		Themes: fakeThemes{st}, Dates: fakeDates{st}, Photos: fakePhotos{st},
		Storage: h.storage, Push: h.push,
		IDTokens: fakeVerifier{claims: map[string]out.IDTokenClaims{}}, Access: fakeAccess{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return h
}

// newUser 는 필름 balance 장을 가진 사용자를 만들고 기기 토큰 "tok-<nick>" 을 등록한다.
func (h *harness) newUser(nick string, balance int) uuid.UUID {
	id := uuid.New()
	h.st.users[id] = user.User{ID: id, Nickname: nick, FilmBalance: balance, Status: user.StatusActive, CreatedAt: h.clock.Now()}
	h.st.ledger = append(h.st.ledger, user.LedgerEntry{ID: uuid.New(), UserID: id, Delta: balance, Reason: user.LedgerSignup})
	h.st.devices["tok-"+nick] = user.Device{UserID: id, Token: "tok-" + nick, Platform: user.PlatformIOS}
	return id
}

// newCouple 은 연결된 두 사용자(alice, bob)를 만든다.
func (h *harness) newCouple() (uuid.UUID, uuid.UUID) {
	a, b := h.newUser("alice", user.SignupFilm), h.newUser("bob", user.SignupFilm)
	h.st.couples = append(h.st.couples, couple.Couple{ID: uuid.New(), UserAID: a, UserBID: b, Status: couple.StatusActive, ConnectedAt: h.clock.Now()})
	return a, b
}
