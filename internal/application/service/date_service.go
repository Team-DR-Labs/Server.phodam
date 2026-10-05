package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

// DateService 는 DateUseCase 구현체다.
type DateService struct{ d Deps }

var _ in.DateUseCase = (*DateService)(nil)

// NewDateService 는 DateService 를 생성한다.
func NewDateService(d Deps) *DateService { return &DateService{d: d} }

// Start 는 새 데이트를 시작한다. 테마와 서로 다른 주제 2개를 랜덤으로 고른다.
func (s *DateService) Start(ctx context.Context, userID uuid.UUID) (dating.View, error) {
	c, err := s.d.requireCouple(ctx, userID)
	if err != nil {
		return dating.View{}, err
	}
	var started dating.Date
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		started, err = s.start(ctx, c, userID)
		return err
	})
	if err != nil {
		return dating.View{}, err
	}
	s.d.notify(ctx, push.TypeDateStarted, started.ID, c.PartnerOf(userID))
	return s.d.buildView(ctx, started, userID)
}

func (s *DateService) start(ctx context.Context, c couple.Couple, userID uuid.UUID) (dating.Date, error) {
	now := s.d.now()
	if err := s.ensureNoActiveDate(ctx, c.ID); err != nil {
		return dating.Date{}, err
	}
	theme, topics, err := s.pickThemeAndTopics(ctx, c.ID)
	if err != nil {
		return dating.Date{}, err
	}
	d := dating.Start(dating.StartParams{
		ID: uuid.New(), CoupleID: c.ID, Theme: theme,
		Starter: userID, Partner: c.PartnerOf(userID),
		StarterTopic: topics[0], PartnerTopic: topics[1], Now: now,
	})
	if err := s.d.Dates.Create(ctx, d); err != nil {
		if errors.Is(err, out.ErrDuplicate) {
			return dating.Date{}, apperr.New(apperr.DateAlreadyInProgress, "date already in progress")
		}
		return dating.Date{}, fmt.Errorf("create date: %w", err)
	}
	return d, nil
}

// ensureNoActiveDate 는 진행 중 데이트가 있으면 거부한다.
// 마감이 지났는데 워커가 아직 만료시키지 않은 데이트는 여기서 즉시 만료시킨다.
func (s *DateService) ensureNoActiveDate(ctx context.Context, coupleID uuid.UUID) error {
	cur, err := s.d.Dates.FindInProgressByCouple(ctx, coupleID)
	if errors.Is(err, out.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find current date: %w", err)
	}
	now := s.d.now()
	if cur.IsActiveAt(now) {
		return apperr.New(apperr.DateAlreadyInProgress, "date already in progress")
	}
	if err := s.d.Dates.Expire(ctx, cur.ID, now); err != nil {
		return fmt.Errorf("expire due date: %w", err)
	}
	return nil
}

func (s *DateService) pickThemeAndTopics(ctx context.Context, coupleID uuid.UUID) (dating.Theme, []dating.Topic, error) {
	themes, err := s.d.Themes.ListThemes(ctx)
	if err != nil {
		return dating.Theme{}, nil, fmt.Errorf("list themes: %w", err)
	}
	hist, err := s.d.Dates.ThemeHistory(ctx, coupleID)
	if err != nil {
		return dating.Theme{}, nil, fmt.Errorf("theme history: %w", err)
	}
	candidates := dating.ThemeCandidates(themes, hist.Used, hist.Last)
	if len(candidates) == 0 {
		return dating.Theme{}, nil, errors.New("no themes seeded")
	}
	theme := candidates[rand.IntN(len(candidates))]
	topics, err := s.d.Themes.ListTopics(ctx, theme.ID)
	if err != nil {
		return dating.Theme{}, nil, fmt.Errorf("list topics: %w", err)
	}
	if len(topics) < 2 {
		return dating.Theme{}, nil, fmt.Errorf("theme %s has fewer than 2 topics", theme.ID)
	}
	perm := rand.Perm(len(topics))
	return theme, []dating.Topic{topics[perm[0]], topics[perm[1]]}, nil
}

// Current 는 진행 중(마감 전) 데이트를 반환한다. 없으면 nil.
func (s *DateService) Current(ctx context.Context, userID uuid.UUID) (*dating.View, error) {
	c, err := s.d.requireCouple(ctx, userID)
	if err != nil {
		return nil, err
	}
	return currentDate(ctx, s.d, c, userID)
}

// Get 은 참여한 데이트를 반환한다.
func (s *DateService) Get(ctx context.Context, userID, dateID uuid.UUID) (dating.View, error) {
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return dating.View{}, err
	}
	d, err := s.d.loadDate(ctx, userID, dateID, false)
	if err != nil {
		return dating.View{}, err
	}
	return s.d.buildView(ctx, d, userID)
}

// Join 은 내 주제를 확인한다 (assigned → joined, 멱등).
func (s *DateService) Join(ctx context.Context, userID, dateID uuid.UUID) (dating.View, error) {
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return dating.View{}, err
	}
	var joined dating.Date
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		d, err := s.d.loadDate(ctx, userID, dateID, true)
		if err != nil {
			return err
		}
		next, changed, err := d.Join(userID, s.d.now())
		if err != nil {
			return err
		}
		if changed {
			me, _ := next.Participant(userID)
			if err := s.d.Dates.UpdateParticipant(ctx, dateID, me); err != nil {
				return fmt.Errorf("update participant: %w", err)
			}
		}
		joined = next
		return nil
	})
	if err != nil {
		return dating.View{}, err
	}
	return s.d.buildView(ctx, joined, userID)
}
