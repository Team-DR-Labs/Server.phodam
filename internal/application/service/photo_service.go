package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/photo"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

// PhotoService 는 PhotoUseCase 구현체다.
type PhotoService struct{ d Deps }

var _ in.PhotoUseCase = (*PhotoService)(nil)

// NewPhotoService 는 PhotoService 를 생성한다.
func NewPhotoService(d Deps) *PhotoService { return &PhotoService{d: d} }

// ReserveShot 은 필름 1장을 차감하고 사진을 예약한 뒤 업로드 URL 을 발급한다 (정책 §6).
func (s *PhotoService) ReserveShot(ctx context.Context, userID, dateID uuid.UUID) (in.ShotReservation, error) {
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return in.ShotReservation{}, err
	}
	var (
		p       photo.Photo
		balance int
	)
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		p, balance, err = s.reserve(ctx, userID, dateID)
		return err
	})
	if err != nil {
		return in.ShotReservation{}, err
	}
	target, err := s.uploadTarget(ctx, p)
	if err != nil {
		return in.ShotReservation{}, err
	}
	return in.ShotReservation{Photo: p, Upload: target, FilmBalance: balance}, nil
}

func (s *PhotoService) reserve(ctx context.Context, userID, dateID uuid.UUID) (photo.Photo, int, error) {
	now := s.d.now()
	d, err := s.d.loadDate(ctx, userID, dateID, true)
	if err != nil {
		return photo.Photo{}, 0, err
	}
	if err := d.CheckCanCapture(userID, now); err != nil {
		return photo.Photo{}, 0, err
	}
	balance, ok, err := s.d.Users.DecrementFilm(ctx, userID)
	if err != nil {
		return photo.Photo{}, 0, fmt.Errorf("decrement film: %w", err)
	}
	if !ok {
		return photo.Photo{}, 0, apperr.New(apperr.FilmExhausted, "no film left")
	}
	p := photo.NewReserved(uuid.New(), dateID, userID, now)
	ref := p.ID
	entry := user.LedgerEntry{ID: uuid.New(), UserID: userID, Delta: -1, Reason: user.LedgerShot, RefID: &ref, CreatedAt: now}
	if err := s.d.Ledger.Append(ctx, entry); err != nil {
		return photo.Photo{}, 0, fmt.Errorf("append ledger: %w", err)
	}
	if err := s.d.Photos.Create(ctx, p); err != nil {
		return photo.Photo{}, 0, fmt.Errorf("create photo: %w", err)
	}
	return p, balance, nil
}

// ReissueUploadURL 은 reserved 사진의 업로드 URL 을 다시 발급한다.
func (s *PhotoService) ReissueUploadURL(ctx context.Context, userID, photoID uuid.UUID) (in.UploadTarget, error) {
	p, err := s.ownedPhoto(ctx, userID, photoID)
	if err != nil {
		return in.UploadTarget{}, err
	}
	if err := s.checkCapture(ctx, userID, p.DateID); err != nil {
		return in.UploadTarget{}, err
	}
	if p.Status != photo.StatusReserved {
		return in.UploadTarget{}, errNotFound("reserved photo")
	}
	return s.uploadTarget(ctx, p)
}

// CompleteUpload 는 temp 객체를 확인하고 uploaded 로 바꾼다 (멱등).
func (s *PhotoService) CompleteUpload(ctx context.Context, userID, photoID uuid.UUID) (photo.Photo, error) {
	p, err := s.ownedPhoto(ctx, userID, photoID)
	if err != nil {
		return photo.Photo{}, err
	}
	switch p.Status {
	case photo.StatusReserved:
	case photo.StatusDeleted:
		return photo.Photo{}, errNotFound("photo")
	default:
		return p, nil
	}
	if err := s.checkCapture(ctx, userID, p.DateID); err != nil {
		return photo.Photo{}, err
	}
	info, err := s.d.Storage.Stat(ctx, out.BucketTemp, p.TempKey)
	if errors.Is(err, out.ErrObjectNotFound) {
		return photo.Photo{}, apperr.New(apperr.PhotoNotUploaded, "photo is not uploaded")
	}
	if err != nil {
		return photo.Photo{}, fmt.Errorf("stat temp object: %w", err)
	}
	if info.Size > photo.MaxSizeBytes {
		if err := s.d.Storage.Remove(ctx, out.BucketTemp, p.TempKey); err != nil {
			return photo.Photo{}, fmt.Errorf("remove oversized object: %w", err)
		}
		return photo.Photo{}, apperr.New(apperr.PhotoInvalid, "photo exceeds 20MB")
	}
	return s.markUploaded(ctx, photoID, info.Size)
}

func (s *PhotoService) markUploaded(ctx context.Context, photoID uuid.UUID, size int64) (photo.Photo, error) {
	var updated photo.Photo
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.d.Photos.GetForUpdate(ctx, photoID)
		if err != nil {
			return fmt.Errorf("lock photo: %w", err)
		}
		if p.Status != photo.StatusReserved {
			updated = p
			return nil
		}
		updated = p.MarkUploaded(size, s.d.now())
		return s.d.Photos.Update(ctx, updated)
	})
	return updated, err
}

// ListMine 은 이 데이트의 내 uploaded/archived 사진을 미리보기 URL 과 함께 반환한다.
func (s *PhotoService) ListMine(ctx context.Context, userID, dateID uuid.UUID) ([]in.PhotoWithURL, error) {
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return nil, err
	}
	if _, err := s.d.loadDate(ctx, userID, dateID, false); err != nil {
		return nil, err
	}
	photos, err := s.d.Photos.ListByOwner(ctx, dateID, userID, photo.StatusUploaded, photo.StatusArchived)
	if err != nil {
		return nil, fmt.Errorf("list photos: %w", err)
	}
	return s.withURLs(ctx, photos)
}

// Receivable 은 제출 후 7일 동안 수령 가능한 내 사진을 반환한다.
func (s *PhotoService) Receivable(ctx context.Context, userID, dateID uuid.UUID) (in.Receivable, error) {
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return in.Receivable{}, err
	}
	d, err := s.d.loadDate(ctx, userID, dateID, false)
	if err != nil {
		return in.Receivable{}, err
	}
	me, _ := d.Participant(userID)
	if !me.CanReceiveAt(s.d.now()) {
		return in.Receivable{}, apperr.New(apperr.ReceiveNotAvailable, "receive not available")
	}
	photos, err := s.d.Photos.ListByOwner(ctx, dateID, userID, photo.StatusUploaded, photo.StatusArchived)
	if err != nil {
		return in.Receivable{}, fmt.Errorf("list photos: %w", err)
	}
	items, err := s.withURLs(ctx, receivableOnly(photos))
	if err != nil {
		return in.Receivable{}, err
	}
	return in.Receivable{ReceiveDeadlineAt: *me.ReceiveDeadlineAt, Items: items}, nil
}

// receivableOnly 는 uploaded 비대표 사진과 archived 대표 사진만 남긴다.
func receivableOnly(photos []photo.Photo) []photo.Photo {
	res := make([]photo.Photo, 0, len(photos))
	for _, p := range photos {
		if (p.Status == photo.StatusUploaded && !p.IsRepresentative) || (p.Status == photo.StatusArchived && p.IsRepresentative) {
			res = append(res, p)
		}
	}
	return res
}

// AckReceived 는 기기 저장 성공을 기록한다. 비대표 사진은 temp 객체를 지운다 (멱등).
func (s *PhotoService) AckReceived(ctx context.Context, userID, photoID uuid.UUID) (photo.Photo, error) {
	p, err := s.ownedPhoto(ctx, userID, photoID)
	if err != nil {
		return photo.Photo{}, err
	}
	if p.IsReceiveAcked() {
		return p, nil
	}
	d, err := s.d.loadDate(ctx, userID, p.DateID, false)
	if err != nil {
		return photo.Photo{}, err
	}
	me, _ := d.Participant(userID)
	if !me.CanReceiveAt(s.d.now()) || (p.Status != photo.StatusUploaded && p.Status != photo.StatusArchived) {
		return photo.Photo{}, apperr.New(apperr.ReceiveNotAvailable, "receive not available")
	}
	if !p.IsRepresentative {
		if err := s.d.Storage.Remove(ctx, out.BucketTemp, p.TempKey); err != nil {
			return photo.Photo{}, fmt.Errorf("remove temp object: %w", err)
		}
	}
	return s.markReceived(ctx, photoID)
}

func (s *PhotoService) markReceived(ctx context.Context, photoID uuid.UUID) (photo.Photo, error) {
	var updated photo.Photo
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		p, err := s.d.Photos.GetForUpdate(ctx, photoID)
		if err != nil {
			return fmt.Errorf("lock photo: %w", err)
		}
		if p.IsReceiveAcked() {
			updated = p
			return nil
		}
		updated = p.MarkReceived(s.d.now())
		return s.d.Photos.Update(ctx, updated)
	})
	return updated, err
}

// ownedPhoto 는 내 사진을 읽는다. 남의 사진이면 NOT_FOUND 다.
func (s *PhotoService) ownedPhoto(ctx context.Context, userID, photoID uuid.UUID) (photo.Photo, error) {
	if _, err := s.d.requireCouple(ctx, userID); err != nil {
		return photo.Photo{}, err
	}
	p, err := s.d.Photos.Get(ctx, photoID)
	if errors.Is(err, out.ErrNotFound) || (err == nil && !p.IsOwnedBy(userID)) {
		return photo.Photo{}, errNotFound("photo")
	}
	if err != nil {
		return photo.Photo{}, fmt.Errorf("get photo: %w", err)
	}
	return p, nil
}

func (s *PhotoService) checkCapture(ctx context.Context, userID, dateID uuid.UUID) error {
	d, err := s.d.loadDate(ctx, userID, dateID, false)
	if err != nil {
		return err
	}
	return d.CheckCanCapture(userID, s.d.now())
}

func (s *PhotoService) uploadTarget(ctx context.Context, p photo.Photo) (in.UploadTarget, error) {
	expires := s.d.now().Add(photo.URLTTL)
	url, err := s.d.Storage.PresignPut(ctx, out.BucketTemp, p.TempKey, photo.ContentType, photo.URLTTL)
	if err != nil {
		return in.UploadTarget{}, fmt.Errorf("presign put: %w", err)
	}
	return in.UploadTarget{
		URL: url, Method: http.MethodPut,
		Headers: map[string]string{"Content-Type": photo.ContentType}, ExpiresAt: expires,
	}, nil
}

func (s *PhotoService) withURLs(ctx context.Context, photos []photo.Photo) ([]in.PhotoWithURL, error) {
	items := make([]in.PhotoWithURL, 0, len(photos))
	for _, p := range photos {
		u, err := presignPhoto(ctx, s.d, p)
		if err != nil {
			return nil, err
		}
		items = append(items, in.PhotoWithURL{Photo: p, URL: u.URL, URLExpiresAt: u.ExpiresAt})
	}
	return items, nil
}
