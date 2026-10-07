package db

import (
	"context"
	"database/sql"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
	"saveboard/migrations"
)

func Migrate(ctx context.Context, database *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, database, migrations.Files)
	if err != nil {
		return err
	}
	if _, err := provider.Up(ctx); err != nil {
		return err
	}
	migrator, err := rivermigrate.New(riverdatabasesql.New(database), nil)
	if err != nil {
		return err
	}
	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	return err
}
