package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Queries struct {
	db dbtx
}

type nullString struct {
	String string
	Valid  bool
}

var ErrLockBusy = errors.New("lock busy")
var ErrNotFound = errors.New("not found")

func New(ctx context.Context, dsn string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	return &Repository{pool: pool}, nil
}

func (r *Repository) Close() {
	r.pool.Close()
}

func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repository) Queries() *Queries {
	return &Queries{db: r.pool}
}

func (r *Repository) InTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	q := &Queries{db: tx}
	if err := fn(q); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback tx: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *Repository) AcquireAdvisoryLock(ctx context.Context, key1, key2 int32) (func(context.Context) error, error) {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}

	var ok bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1, $2)`, key1, key2).Scan(&ok); err != nil {
		conn.Release()
		return nil, fmt.Errorf("acquire advisory lock: %w", err)
	}
	if !ok {
		conn.Release()
		return nil, ErrLockBusy
	}

	unlock := func(ctx context.Context) error {
		defer conn.Release()
		var released bool
		if err := conn.QueryRow(ctx, `SELECT pg_advisory_unlock($1, $2)`, key1, key2).Scan(&released); err != nil {
			return fmt.Errorf("release advisory lock: %w", err)
		}
		if !released {
			return fmt.Errorf("advisory lock was not held")
		}
		return nil
	}

	return unlock, nil
}
