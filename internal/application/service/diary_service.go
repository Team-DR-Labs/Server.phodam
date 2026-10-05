package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/diary"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
)

// DiaryService 는 DiaryUseCase 구현체다.
type DiaryService struct{ d Deps }

var _ in.DiaryUseCase = (*DiaryService)(nil)

// NewDiaryService 는 DiaryService 를 생성한다.
func NewDiaryService(d Deps) *DiaryService { return &DiaryService{d: d} }

// List 는 내가 제출한 데이트를 started_at 내림차순으로 반환한다.
func (s *DiaryService) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) (diary.Page, error) {
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return diary.Page{}, err
	}
	if limit == 0 {
		limit = diary.DefaultLimit
	}
	if limit < 1 || limit > diary.MaxLimit {
		return diary.Page{}, errValidation("limit must be 1-50")
	}
	after, err := decodeCursor(cursor)
	if err != nil {
		return diary.Page{}, err
	}
	dates, err := s.d.Dates.ListSubmittedByUser(ctx, userID, after, limit+1)
	if err != nil {
		return diary.Page{}, fmt.Errorf("list diaries: %w", err)
	}
	page := diary.Page{Items: make([]diary.ListItem, 0, min(len(dates), limit))}
	if len(dates) > limit {
		last := dates[limit-1]
		next := encodeCursor(diary.Cursor{StartedAt: last.StartedAt, DateID: last.ID})
		page.NextCursor = &next
		dates = dates[:limit]
	}
	for _, d := range dates {
		item, ok, err := s.listItem(ctx, d, userID)
		if err != nil {
			return diary.Page{}, err
		}
		if ok {
			page.Items = append(page.Items, item)
		}
	}
	return page, nil
}

func (s *DiaryService) listItem(ctx context.Context, d dating.Date, viewer uuid.UUID) (diary.ListItem, bool, error) {
	vis, ok := diary.VisibilityFor(d, viewer, s.d.now())
	if !ok {
		return diary.ListItem{}, false, nil
	}
	me, _ := d.Participant(viewer)
	thumb, err := s.representativeURL(ctx, d, me)
	if err != nil {
		return diary.ListItem{}, false, err
	}
	return diary.ListItem{
		DateID: d.ID, LocalDate: diary.LocalDate(d.StartedAt), Theme: d.Theme,
		Visibility: vis, StartedAt: d.StartedAt, Thumbnail: thumb,
	}, true, nil
}

// Get 은 일기 상세를 반환한다. 상대 내용은 shared(revealed)일 때만 넣는다.
func (s *DiaryService) Get(ctx context.Context, userID, dateID uuid.UUID) (diary.Detail, error) {
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return diary.Detail{}, err
	}
	d, err := s.d.loadDate(ctx, userID, dateID, false)
	if err != nil {
		return diary.Detail{}, err
	}
	vis, ok := diary.VisibilityFor(d, userID, s.d.now())
	if !ok {
		return diary.Detail{}, errNotFound("diary")
	}
	me, _ := d.Participant(userID)
	entries := make([]diary.Entry, 0, 2)
	mine, err := s.entry(ctx, d, me, true)
	if err != nil {
		return diary.Detail{}, err
	}
	entries = append(entries, mine)
	if vis == diary.VisibilityShared {
		partner, _ := d.Partner(userID)
		theirs, err := s.entry(ctx, d, partner, false)
		if err != nil {
			return diary.Detail{}, err
		}
		entries = append(entries, theirs)
	}
	return diary.Detail{
		DateID: d.ID, LocalDate: diary.LocalDate(d.StartedAt), Theme: d.Theme,
		Visibility: vis, StartedAt: d.StartedAt, Entries: entries,
	}, nil
}

func (s *DiaryService) entry(ctx context.Context, d dating.Date, p dating.Participant, isMe bool) (diary.Entry, error) {
	author, err := s.d.Users.Get(ctx, p.UserID)
	if err != nil {
		return diary.Entry{}, fmt.Errorf("get author: %w", err)
	}
	u, err := s.representativeURL(ctx, d, p)
	if err != nil {
		return diary.Entry{}, err
	}
	return diary.Entry{
		Author: author.Profile(), IsMe: isMe, Topic: p.Topic, Caption: p.Caption,
		Photo: u, SubmittedAt: *p.SubmittedAt,
	}, nil
}

// representativeURL 은 permanent 대표 사진 URL 이다. 수령(received) 여부와 무관하게 남아 있다.
func (s *DiaryService) representativeURL(ctx context.Context, d dating.Date, p dating.Participant) (diary.URL, error) {
	if p.RepresentativePhotoID == nil {
		return diary.URL{}, fmt.Errorf("participant %s has no representative photo", p.UserID)
	}
	key := photo.PermanentKey(d.ID, p.UserID, *p.RepresentativePhotoID)
	return presignKey(ctx, s.d, out.BucketPermanent, key)
}

func encodeCursor(c diary.Cursor) string {
	raw := strconv.FormatInt(c.StartedAt.UnixMicro(), 10) + "|" + c.DateID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*diary.Cursor, error) {
	if s == "" {
		return nil, nil
	}
	invalid := errValidation("cursor is invalid")
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, invalid
	}
	micros, idStr, ok := strings.Cut(string(raw), "|")
	if !ok {
		return nil, invalid
	}
	us, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return nil, invalid
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, invalid
	}
	return &diary.Cursor{StartedAt: time.UnixMicro(us).UTC(), DateID: id}, nil
}
