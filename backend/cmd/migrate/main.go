package main

import (
	"context"
	"os"
	"saveboard/internal/platform/config"
	"saveboard/internal/platform/db"
	"saveboard/internal/platform/telemetry"
	"time"
)

func main() {
	log := telemetry.Init("migrate")
	if err := run(); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}
	log.Info("application and River migrations complete")
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cfg, err := config.Load("migrate")
	if err != nil {
		return err
	}
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	return db.Migrate(ctx, database.DB)
}
