package db

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX is the minimal interface satisfied by both *pgxpool.Pool and pgx.Tx.
//
// This is the key to making repos transaction-aware without coupling them to
// either the pool or a specific transaction type. The service layer decides
// whether to pass a pool (for standalone reads) or a tx (for atomic writes).
//
// Never add methods to this interface unless both Pool and Tx implement them.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
