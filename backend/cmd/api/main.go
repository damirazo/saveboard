package main

import (
	"context"
	"os"
	"os/signal"
	"saveboard/internal/platform/config"
	"saveboard/internal/platform/db"
	"saveboard/internal/platform/httpserver"
	"saveboard/internal/platform/telemetry"
	"saveboard/internal/transport/httpapi"
	"syscall"
	"time"
)

func main() {
	log := telemetry.Init("api")
	if err := run(); err != nil {
		log.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	cfg, err := config.Load("api")
	if err != nil {
		return err
	}
	connectCtx, stop := context.WithTimeout(ctx, 10*time.Second)
	defer stop()
	database, err := db.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	handler, _ := httpapi.New(httpapi.Dependencies{Ping: database.PingContext})
	return httpserver.Run(ctx, cfg.HTTPAddr, handler)
}
