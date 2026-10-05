package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/Team-DR-Labs/Server.phodam/internal/application/port/out"
)

type txKey struct{}

// TxManager 는 out.TxManager 구현체다. ctx 에 *gorm.DB 트랜잭션을 실어 리포지토리에 전달한다.
type TxManager struct {
	db *gorm.DB
}

var _ out.TxManager = (*TxManager)(nil)

// NewTxManager 는 TxManager 를 생성한다.
func NewTxManager(db *gorm.DB) *TxManager { return &TxManager{db: db} }

// WithinTx 는 fn 을 트랜잭션 안에서 실행한다. 이미 트랜잭션 안이면 그대로 합류한다.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// conn 은 ctx 의 트랜잭션이 있으면 그것을, 없으면 기본 연결을 반환한다.
func conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}

const pgUniqueViolation = "23505"

// translate 는 드라이버 에러를 포트 에러로 바꾼다.
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return out.ErrDuplicate
	}
	return err
}

// requireRow 는 UPDATE 결과 영향받은 행이 없으면 ErrNotFound 를 반환한다.
func requireRow(res *gorm.DB) error {
	if res.Error != nil {
		return translate(res.Error)
	}
	if res.RowsAffected == 0 {
		return out.ErrNotFound
	}
	return nil
}

// idRow 는 RETURNING id 같은 단일 uuid 컬럼 조회 결과다.
// gorm 은 uuid.UUID([16]byte) 를 바로 Scan 하면 바이트 배열로 다루므로 구조체 필드로 받는다.
type idRow struct {
	ID uuid.UUID
}

func ids(rows []idRow) []uuid.UUID {
	res := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		res = append(res, r.ID)
	}
	return res
}
