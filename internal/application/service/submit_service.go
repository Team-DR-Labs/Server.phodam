package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/dating"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

// submitResult 는 제출 트랜잭션의 결과다.
type submitResult struct {
	date     dating.Date
	revealed bool
	rep      photo.Photo
	dropped  []photo.Photo
}

// Submit 은 대표 사진과 글을 제출한다 (정책 §7).
// 객체 복사 → 상태 변경 트랜잭션 → temp 정리 → 푸시 순서다.
func (s *DateService) Submit(ctx context.Context, userID, dateID, photoID uuid.UUID, caption *string) (dating.View, error) {
	text, err := dating.NormalizeCaption(caption)
	if err != nil {
		return dating.View{}, err
	}
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return dating.View{}, err
	}
	rep, err := s.precheckSubmit(ctx, userID, dateID, photoID)
	if err != nil {
		return dating.View{}, err
	}
	permKey := photo.PermanentKey(rep.DateID, rep.OwnerID, rep.ID)
	if err := s.d.Storage.Copy(ctx, out.BucketTemp, rep.TempKey, out.BucketPermanent, permKey); err != nil {
		return dating.View{}, fmt.Errorf("copy to permanent: %w", err)
	}

	var res submitResult
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		res, err = s.submitTx(ctx, userID, dateID, photoID, text)
		return err
	})
	if err != nil {
		s.discardPermanentCopy(ctx, photoID, permKey)
		return dating.View{}, err
	}

	s.purgeTemp(ctx, res.rep, res.dropped)
	if res.revealed {
		s.d.notify(ctx, push.TypeDateRevealed, dateID, userIDs(res.date)...)
	} else {
		partner, _ := res.date.Partner(userID)
		s.d.notify(ctx, push.TypePartnerSubmitted, dateID, partner.UserID)
	}
	return s.d.buildView(ctx, res.date, userID)
}

// precheckSubmit 은 객체 복사 전에 제출 가능 여부와 대표 사진을 확인한다.
func (s *DateService) precheckSubmit(ctx context.Context, userID, dateID, photoID uuid.UUID) (photo.Photo, error) {
	d, err := s.d.loadDate(ctx, userID, dateID, false)
	if err != nil {
		return photo.Photo{}, err
	}
	if err := d.CheckCanCapture(userID, s.d.now()); err != nil {
		return photo.Photo{}, err
	}
	return s.loadRepresentative(ctx, userID, dateID, photoID, false)
}

func (s *DateService) loadRepresentative(ctx context.Context, userID, dateID, photoID uuid.UUID, lock bool) (photo.Photo, error) {
	get := s.d.Photos.Get
	if lock {
		get = s.d.Photos.GetForUpdate
	}
	p, err := get(ctx, photoID)
	if errors.Is(err, out.ErrNotFound) || (err == nil && (!p.IsOwnedBy(userID) || p.DateID != dateID)) {
		return photo.Photo{}, errNotFound("photo")
	}
	if err != nil {
		return photo.Photo{}, fmt.Errorf("get photo: %w", err)
	}
	if p.Status != photo.StatusUploaded {
		return photo.Photo{}, apperr.New(apperr.PhotoNotUploaded, "photo is not uploaded")
	}
	return p, nil
}

func (s *DateService) submitTx(ctx context.Context, userID, dateID, photoID uuid.UUID, caption *string) (submitResult, error) {
	now := s.d.now()
	d, err := s.d.loadDate(ctx, userID, dateID, true)
	if err != nil {
		return submitResult{}, err
	}
	p, err := s.loadRepresentative(ctx, userID, dateID, photoID, true)
	if err != nil {
		return submitResult{}, err
	}
	next, revealed, err := d.Submit(userID, photoID, caption, now)
	if err != nil {
		return submitResult{}, err
	}
	rep := p.Archive()
	if err := s.d.Photos.Update(ctx, rep); err != nil {
		return submitResult{}, fmt.Errorf("archive photo: %w", err)
	}
	me, _ := next.Participant(userID)
	if err := s.d.Dates.UpdateParticipant(ctx, dateID, me); err != nil {
		return submitResult{}, fmt.Errorf("update participant: %w", err)
	}
	dropped, err := s.d.Photos.DeleteReserved(ctx, dateID, userID, now)
	if err != nil {
		return submitResult{}, fmt.Errorf("delete reserved photos: %w", err)
	}
	if revealed {
		if err := s.d.Dates.MarkRevealed(ctx, dateID, now); err != nil {
			return submitResult{}, fmt.Errorf("reveal date: %w", err)
		}
	}
	return submitResult{date: next, revealed: revealed, rep: rep, dropped: dropped}, nil
}

// discardPermanentCopy 는 트랜잭션 실패 시 permanent 사본을 지운다.
// 동시 제출로 이미 같은 사진이 보관된 경우에는 지우지 않는다.
func (s *DateService) discardPermanentCopy(ctx context.Context, photoID uuid.UUID, key string) {
	if p, err := s.d.Photos.Get(ctx, photoID); err == nil && p.Status == photo.StatusArchived {
		return
	}
	if err := s.d.Storage.Remove(ctx, out.BucketPermanent, key); err != nil {
		s.d.Logger.WarnContext(ctx, "remove permanent copy", slog.String("key", key), slog.Any("error", err))
	}
}

// purgeTemp 는 대표 사진의 temp 사본과 업로드되지 않은 샷의 잔여 객체를 지운다.
// 실패해도 응답은 성공이며 워커가 다시 시도한다.
func (s *DateService) purgeTemp(ctx context.Context, rep photo.Photo, dropped []photo.Photo) {
	if err := s.d.Storage.Remove(ctx, out.BucketTemp, rep.TempKey); err != nil {
		s.d.Logger.WarnContext(ctx, "remove representative temp", slog.Any("error", err))
	} else if err := s.d.Photos.MarkTempPurged(ctx, rep.ID, s.d.now()); err != nil {
		s.d.Logger.WarnContext(ctx, "mark temp purged", slog.Any("error", err))
	}
	for _, p := range dropped {
		if err := s.d.Storage.Remove(ctx, out.BucketTemp, p.TempKey); err != nil {
			s.d.Logger.WarnContext(ctx, "remove dropped shot", slog.Any("error", err))
		}
	}
}

func userIDs(d dating.Date) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(d.Participants))
	for _, p := range d.Participants {
		ids = append(ids, p.UserID)
	}
	return ids
}
