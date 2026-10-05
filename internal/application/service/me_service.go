package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// MeService 는 MeUseCase 구현체다.
type MeService struct{ d Deps }

var _ in.MeUseCase = (*MeService)(nil)

// NewMeService 는 MeService 를 생성한다.
func NewMeService(d Deps) *MeService { return &MeService{d: d} }

// Get 은 프로필, 필름 잔량, 커플, 진행 중 데이트를 모아 반환한다.
func (s *MeService) Get(ctx context.Context, userID uuid.UUID) (in.Me, error) {
	u, err := s.d.Users.Get(ctx, userID)
	if errors.Is(err, out.ErrNotFound) {
		return in.Me{}, apperr.New(apperr.Unauthorized, "user not found")
	}
	if err != nil {
		return in.Me{}, fmt.Errorf("get user: %w", err)
	}
	me := in.Me{User: u.Profile(), FilmBalance: u.FilmBalance}

	c, err := s.d.Couples.FindActiveByUser(ctx, userID)
	if errors.Is(err, out.ErrNotFound) {
		return me, nil
	}
	if err != nil {
		return in.Me{}, fmt.Errorf("find couple: %w", err)
	}
	cv, err := coupleView(ctx, s.d, c, userID)
	if err != nil {
		return in.Me{}, err
	}
	me.Couple = &cv

	current, err := currentDate(ctx, s.d, c, userID)
	if err != nil {
		return in.Me{}, err
	}
	me.CurrentDate = current
	return me, nil
}

// UpdateNickname 은 닉네임을 바꾸고 최신 상태를 반환한다.
func (s *MeService) UpdateNickname(ctx context.Context, userID uuid.UUID, nickname string) (in.Me, error) {
	n, err := user.ValidateNickname(nickname)
	if err != nil {
		return in.Me{}, err
	}
	if err := s.d.Users.UpdateNickname(ctx, userID, n); err != nil {
		return in.Me{}, fmt.Errorf("update nickname: %w", err)
	}
	return s.Get(ctx, userID)
}

// RegisterDevice 는 FCM 토큰을 등록한다. 다른 사용자에게 등록된 토큰이면 소유자를 옮긴다.
func (s *MeService) RegisterDevice(ctx context.Context, userID uuid.UUID, token, platform string) error {
	dev, err := user.NewDevice(userID, token, platform)
	if err != nil {
		return err
	}
	if err := s.d.Devices.Upsert(ctx, dev); err != nil {
		return fmt.Errorf("upsert device: %w", err)
	}
	return nil
}

func coupleView(ctx context.Context, d Deps, c couple.Couple, viewer uuid.UUID) (in.CoupleView, error) {
	partner, err := d.Users.Get(ctx, c.PartnerOf(viewer))
	if err != nil {
		return in.CoupleView{}, fmt.Errorf("get partner: %w", err)
	}
	return in.CoupleView{ID: c.ID, Partner: partner.Profile(), ConnectedAt: c.ConnectedAt}, nil
}

// currentDate 는 커플의 진행 중(마감 전) 데이트 View 를 반환한다. 없으면 nil.
func currentDate(ctx context.Context, d Deps, c couple.Couple, viewer uuid.UUID) (*dating.View, error) {
	date, err := d.Dates.FindInProgressByCouple(ctx, c.ID)
	if errors.Is(err, out.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find current date: %w", err)
	}
	if !date.IsActiveAt(d.now()) {
		return nil, nil
	}
	v, err := d.buildView(ctx, date, viewer)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
