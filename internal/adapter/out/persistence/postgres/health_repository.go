package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
)

// HealthRepository 는 out.DatabasePinger 구현체다.
type HealthRepository struct {
	db *gorm.DB
}

var _ out.DatabasePinger = (*HealthRepository)(nil)

// NewHealthRepository 는 HealthRepository 를 생성한다.
func NewHealthRepository(db *gorm.DB) *HealthRepository {
	return &HealthRepository{db: db}
}

// Ping 은 DB 연결을 확인한다.
func (r *HealthRepository) Ping(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}
