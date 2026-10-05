package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/in"
	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
	"github.com/Team-DR-Labs/Server.phodam/internal/domain/user"
)

const (
	grantMaxShots    = 240
	grantReasonRunes = 100
)

// AdminService 는 AdminUseCase 구현체다.
type AdminService struct{ d Deps }

var _ in.AdminUseCase = (*AdminService)(nil)

// NewAdminService 는 AdminService 를 생성한다.
func NewAdminService(d Deps) *AdminService { return &AdminService{d: d} }

// GrantFilm 은 필름을 수동 지급하고 원장에 admin_grant 로 기록한다.
func (s *AdminService) GrantFilm(ctx context.Context, userID uuid.UUID, shots int, reason *string) (int, error) {
	if shots < 1 || shots > grantMaxShots {
		return 0, errValidation("shots must be 1-240")
	}
	memo, err := grantMemo(reason)
	if err != nil {
		return 0, err
	}
	var balance int
	err = s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		b, err := s.d.Users.AddFilm(ctx, userID, shots)
		if errors.Is(err, out.ErrNotFound) {
			return errNotFound("user")
		}
		if err != nil {
			return fmt.Errorf("add film: %w", err)
		}
		balance = b
		entry := user.LedgerEntry{
			ID: uuid.New(), UserID: userID, Delta: shots, Reason: user.LedgerAdminGrant,
			Memo: memo, CreatedAt: s.d.now(),
		}
		return s.d.Ledger.Append(ctx, entry)
	})
	return balance, err
}

func grantMemo(reason *string) (*string, error) {
	if reason == nil {
		return nil, nil
	}
	r := strings.TrimSpace(*reason)
	if r == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(r) > grantReasonRunes {
		return nil, errValidation("reason must be at most 100 characters")
	}
	return &r, nil
}
