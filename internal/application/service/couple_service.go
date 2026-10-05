package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/apperr"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/couple"
)

const inviteCreateAttempts = 3

// CoupleService 는 CoupleUseCase 구현체다.
type CoupleService struct{ d Deps }

var _ in.CoupleUseCase = (*CoupleService)(nil)

// NewCoupleService 는 CoupleService 를 생성한다.
func NewCoupleService(d Deps) *CoupleService { return &CoupleService{d: d} }

// Get 은 내 커플을 반환한다. 없으면 COUPLE_REQUIRED.
func (s *CoupleService) Get(ctx context.Context, userID uuid.UUID) (in.CoupleView, error) {
	c, err := s.d.requireCouple(ctx, userID)
	if err != nil {
		return in.CoupleView{}, err
	}
	return coupleView(ctx, s.d, c, userID)
}

// CreateInvite 는 새 초대 코드를 발급하고 내가 만든 이전 미사용 코드를 무효화한다.
func (s *CoupleService) CreateInvite(ctx context.Context, userID uuid.UUID) (in.Invite, error) {
	if err := s.ensureSingle(ctx, userID); err != nil {
		return in.Invite{}, err
	}
	for attempt := 0; attempt < inviteCreateAttempts; attempt++ {
		inv, err := s.createInvite(ctx, userID)
		if errors.Is(err, out.ErrDuplicate) {
			continue
		}
		return inv, err
	}
	return in.Invite{}, errors.New("create invite: code collision")
}

func (s *CoupleService) createInvite(ctx context.Context, userID uuid.UUID) (in.Invite, error) {
	code, err := newInviteCode()
	if err != nil {
		return in.Invite{}, err
	}
	now := s.d.now()
	inv := couple.Invite{ID: uuid.New(), Code: code, CreatorID: userID, ExpiresAt: now.Add(couple.InviteTTL), CreatedAt: now}
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		// 같은 사용자의 동시 발급을 직렬화해 유효한 코드가 하나만 남게 한다.
		if err := s.d.Users.LockForUpdate(ctx, userID); err != nil {
			return fmt.Errorf("lock user: %w", err)
		}
		if err := s.d.Invites.RevokeUnused(ctx, userID, now); err != nil {
			return fmt.Errorf("revoke invites: %w", err)
		}
		return s.d.Invites.Create(ctx, inv)
	})
	if err != nil {
		return in.Invite{}, err
	}
	return in.Invite{Code: inv.Code, ExpiresAt: inv.ExpiresAt}, nil
}

// Join 은 초대 코드로 커플을 연결한다.
func (s *CoupleService) Join(ctx context.Context, userID uuid.UUID, rawCode string) (in.CoupleView, error) {
	code, ok := couple.NormalizeCode(rawCode)
	if !ok {
		return in.CoupleView{}, errValidation("code must be 8 characters")
	}
	var created couple.Couple
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.join(ctx, userID, code)
		return err
	})
	if err != nil {
		return in.CoupleView{}, err
	}
	return coupleView(ctx, s.d, created, userID)
}

func (s *CoupleService) join(ctx context.Context, userID uuid.UUID, code string) (couple.Couple, error) {
	invalid := apperr.New(apperr.InviteInvalid, "invite code is invalid")
	now := s.d.now()
	// 이미 연결된 사용자는 코드 상태와 관계없이 COUPLE_ALREADY_CONNECTED 다.
	if err := s.ensureSingle(ctx, userID); err != nil {
		return couple.Couple{}, err
	}
	inv, err := s.d.Invites.FindByCodeForUpdate(ctx, code)
	if errors.Is(err, out.ErrNotFound) {
		return couple.Couple{}, invalid
	}
	if err != nil {
		return couple.Couple{}, fmt.Errorf("find invite: %w", err)
	}
	if !inv.UsableAt(now) || inv.CreatorID == userID {
		return couple.Couple{}, invalid
	}
	if err := s.d.Users.LockForUpdate(ctx, userID, inv.CreatorID); err != nil {
		return couple.Couple{}, fmt.Errorf("lock users: %w", err)
	}
	for _, id := range []uuid.UUID{userID, inv.CreatorID} {
		if err := s.ensureSingle(ctx, id); err != nil {
			return couple.Couple{}, err
		}
	}
	c := couple.Couple{ID: uuid.New(), UserAID: inv.CreatorID, UserBID: userID, Status: couple.StatusActive, ConnectedAt: now}
	if err := s.d.Couples.Create(ctx, c); err != nil {
		if errors.Is(err, out.ErrDuplicate) {
			return couple.Couple{}, apperr.New(apperr.CoupleAlreadyConnected, "already connected")
		}
		return couple.Couple{}, fmt.Errorf("create couple: %w", err)
	}
	if err := s.d.Invites.MarkUsed(ctx, inv.ID, userID, now); err != nil {
		return couple.Couple{}, fmt.Errorf("mark invite used: %w", err)
	}
	if err := s.d.Invites.RevokeUnused(ctx, userID, now); err != nil {
		return couple.Couple{}, fmt.Errorf("revoke joiner invites: %w", err)
	}
	return c, nil
}

// ensureSingle 은 사용자에게 활성 커플이 없는지 확인한다.
func (s *CoupleService) ensureSingle(ctx context.Context, userID uuid.UUID) error {
	_, err := s.d.Couples.FindActiveByUser(ctx, userID)
	if err == nil {
		return apperr.New(apperr.CoupleAlreadyConnected, "already connected")
	}
	if errors.Is(err, out.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("find couple: %w", err)
}

func newInviteCode() (string, error) {
	alphabet := couple.InviteCodeAlphabet
	max := big.NewInt(int64(len(alphabet)))
	b := make([]byte, couple.InviteCodeLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("generate invite code: %w", err)
		}
		b[i] = alphabet[n.Int64()]
	}
	return string(b), nil
}
