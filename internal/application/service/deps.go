package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

// Deps 는 서비스들이 공유하는 아웃바운드 포트 묶음이다. cmd/server 와 테스트가 채운다.
type Deps struct {
	Clock      out.Clock
	Tx         out.TxManager
	Users      out.UserRepository
	Identities out.IdentityRepository
	Ledger     out.FilmLedgerRepository
	Refresh    out.RefreshTokenRepository
	Devices    out.DeviceRepository
	Couples    out.CoupleRepository
	Invites    out.InviteRepository
	Themes     out.ThemeRepository
	Dates      out.DateRepository
	Photos     out.PhotoRepository
	Storage    out.ObjectStorage
	Push       out.PushSender
	IDTokens   out.IDTokenVerifier
	Access     out.AccessTokenIssuer
	Logger     *slog.Logger
}

func (d Deps) now() time.Time { return d.Clock.Now() }

// requireCouple 은 활성 커플을 반환한다. 없으면 COUPLE_REQUIRED.
func (d Deps) requireCouple(ctx context.Context, userID uuid.UUID) (couple.Couple, error) {
	c, err := d.Couples.FindActiveByUser(ctx, userID)
	if errors.Is(err, out.ErrNotFound) {
		return couple.Couple{}, apperr.New(apperr.CoupleRequired, "couple required")
	}
	if err != nil {
		return couple.Couple{}, fmt.Errorf("find couple: %w", err)
	}
	return c, nil
}

// loadDate 는 viewer 가 참여한 데이트를 읽는다. 참여자가 아니면 NOT_FOUND.
func (d Deps) loadDate(ctx context.Context, viewer, dateID uuid.UUID, lock bool) (dating.Date, error) {
	get := d.Dates.Get
	if lock {
		get = d.Dates.GetForUpdate
	}
	date, err := get(ctx, dateID)
	if errors.Is(err, out.ErrNotFound) {
		return dating.Date{}, errNotFound("date")
	}
	if err != nil {
		return dating.Date{}, fmt.Errorf("get date: %w", err)
	}
	if !date.HasParticipant(viewer) {
		return dating.Date{}, errNotFound("date")
	}
	return date, nil
}

// buildView 는 viewer 시점의 데이트 응답을 만든다.
func (d Deps) buildView(ctx context.Context, date dating.Date, viewer uuid.UUID) (dating.View, error) {
	partner, _ := date.Partner(viewer)
	pu, err := d.Users.Get(ctx, partner.UserID)
	if err != nil {
		return dating.View{}, fmt.Errorf("get partner: %w", err)
	}
	shots, err := d.Photos.CountByOwner(ctx, date.ID, viewer)
	if err != nil {
		return dating.View{}, fmt.Errorf("count shots: %w", err)
	}
	return date.ViewFor(viewer, pu.Profile(), shots, d.now()), nil
}

// notify 는 사용자들의 기기로 푸시를 보낸다. 실패는 로그만 남긴다 (정책 §12).
func (d Deps) notify(ctx context.Context, t push.Type, dateID uuid.UUID, userIDs ...uuid.UUID) {
	tokens, err := d.Devices.ListTokens(ctx, userIDs...)
	if err != nil {
		d.Logger.WarnContext(ctx, "list device tokens", slog.Any("error", err))
		return
	}
	if len(tokens) == 0 {
		return
	}
	invalid, err := d.Push.Send(ctx, tokens, push.NewMessage(t, dateID))
	if err != nil {
		d.Logger.WarnContext(ctx, "send push", slog.String("type", string(t)), slog.Any("error", err))
	}
	if len(invalid) > 0 {
		if err := d.Devices.DeleteTokens(ctx, invalid...); err != nil {
			d.Logger.WarnContext(ctx, "delete invalid tokens", slog.Any("error", err))
		}
	}
}

func errNotFound(what string) error {
	return apperr.New(apperr.NotFound, what+" not found")
}

func errValidation(msg string) error {
	return apperr.New(apperr.ValidationFailed, msg)
}
