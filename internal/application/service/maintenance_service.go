package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/push"
)

// maintenanceBatch 는 한 주기에 단계별로 처리하는 최대 건수다.
const maintenanceBatch = 100

// MaintenanceService 는 정책 §11 주기 작업을 수행한다.
type MaintenanceService struct{ d Deps }

var _ in.MaintenanceUseCase = (*MaintenanceService)(nil)

// NewMaintenanceService 는 MaintenanceService 를 생성한다.
func NewMaintenanceService(d Deps) *MaintenanceService { return &MaintenanceService{d: d} }

// RunOnce 는 만료 → 만료 데이트 사진 정리 → 수령 기한 정리 → 임박 알림 → temp 잔여 정리 순으로 한 번 실행한다.
// 단계별 실패는 로그만 남기고 다음 단계로 넘어간다.
func (s *MaintenanceService) RunOnce(ctx context.Context) {
	steps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"expire_dates", s.expireDates},
		{"purge_expired_photos", s.purgeExpiredPhotos},
		{"purge_receive_overdue", s.purgeReceiveOverdue},
		{"remind_deadline", s.remindDeadline},
		{"purge_archived_temp", s.purgeArchivedTemp},
	}
	for _, st := range steps {
		if err := st.fn(ctx); err != nil {
			s.d.Logger.ErrorContext(ctx, "maintenance step failed", slog.String("step", st.name), slog.Any("error", err))
		}
	}
}

func (s *MaintenanceService) expireDates(ctx context.Context) error {
	return s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		ids, err := s.d.Dates.ExpireDue(ctx, s.d.now(), maintenanceBatch)
		if err != nil {
			return fmt.Errorf("expire dates: %w", err)
		}
		if len(ids) > 0 {
			s.d.Logger.InfoContext(ctx, "dates expired", slog.Int("count", len(ids)))
		}
		return nil
	})
}

// purgeExpiredPhotos 는 만료된 데이트에서 미제출 참여자의 reserved/uploaded 사진을 지운다.
func (s *MaintenanceService) purgeExpiredPhotos(ctx context.Context) error {
	photos, err := s.d.Photos.ListExpiredLeftovers(ctx, maintenanceBatch)
	if err != nil {
		return fmt.Errorf("list expired leftovers: %w", err)
	}
	return s.purgePhotos(ctx, photos, photo.StatusReserved, photo.StatusUploaded)
}

// purgeReceiveOverdue 는 수령 기한이 지난 uploaded 비대표 사진을 지운다. archived 는 지우지 않는다.
func (s *MaintenanceService) purgeReceiveOverdue(ctx context.Context) error {
	photos, err := s.d.Photos.ListReceiveOverdue(ctx, s.d.now(), maintenanceBatch)
	if err != nil {
		return fmt.Errorf("list receive overdue: %w", err)
	}
	return s.purgePhotos(ctx, photos, photo.StatusUploaded)
}

// purgePhotos 는 temp 객체를 지우고, 그동안 상태가 바뀌지 않은 사진만 deleted 로 바꾼다.
// 객체 삭제는 DB 잠금 밖에서 한다. 삭제에 실패한 사진은 상태를 바꾸지 않아 다음 주기에 다시 시도된다.
func (s *MaintenanceService) purgePhotos(ctx context.Context, photos []photo.Photo, from ...photo.Status) error {
	for _, p := range photos {
		if err := s.d.Storage.Remove(ctx, out.BucketTemp, p.TempKey); err != nil {
			s.d.Logger.WarnContext(ctx, "remove photo object", slog.String("photo_id", p.ID.String()), slog.Any("error", err))
			continue
		}
		if _, err := s.d.Photos.MarkDeleted(ctx, p.ID, s.d.now(), from...); err != nil {
			return fmt.Errorf("mark photo deleted: %w", err)
		}
	}
	return nil
}

// remindDeadline 은 마감 6시간 전 미제출 참여자에게 한 번만 알림을 보낸다.
func (s *MaintenanceService) remindDeadline(ctx context.Context) error {
	var reminders []out.Reminder
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		reminders, err = s.d.Dates.ClaimReminders(ctx, s.d.now(), maintenanceBatch)
		return err
	})
	if err != nil {
		return fmt.Errorf("claim reminders: %w", err)
	}
	for _, r := range reminders {
		if len(r.UserIDs) > 0 {
			s.d.notify(ctx, push.TypeDeadlineSoon, r.DateID, r.UserIDs...)
		}
	}
	return nil
}

// purgeArchivedTemp 는 대표 사진의 temp 사본이 남아 있으면 다시 지운다.
func (s *MaintenanceService) purgeArchivedTemp(ctx context.Context) error {
	photos, err := s.d.Photos.ListArchivedWithTemp(ctx, maintenanceBatch)
	if err != nil {
		return fmt.Errorf("list archived photos: %w", err)
	}
	for _, p := range photos {
		if err := s.d.Storage.Remove(ctx, out.BucketTemp, p.TempKey); err != nil {
			s.d.Logger.WarnContext(ctx, "remove archived temp", slog.String("photo_id", p.ID.String()), slog.Any("error", err))
			continue
		}
		if err := s.d.Photos.MarkTempPurged(ctx, p.ID, s.d.now()); err != nil {
			return fmt.Errorf("mark temp purged: %w", err)
		}
	}
	return nil
}
