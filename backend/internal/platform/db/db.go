package db

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"time"
)

func Open(ctx context.Context, dsn string) (*bun.DB, error) {
	sqldb, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(10)
	sqldb.SetMaxIdleConns(5)
	sqldb.SetConnMaxLifetime(30 * time.Minute)
	if err := sqldb.PingContext(ctx); err != nil {
		_ = sqldb.Close()
		return nil, err
	}
	return bun.NewDB(sqldb, pgdialect.New()), nil
}
