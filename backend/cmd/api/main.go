package main

import (
	"context"
	"os"
	"os/signal"
	"saveboard/internal/identity"
	"saveboard/internal/notifications"
	"saveboard/internal/platform/blobstore"
	"saveboard/internal/platform/config"
	"saveboard/internal/platform/db"
	"saveboard/internal/platform/httpserver"
	"saveboard/internal/platform/telemetry"
	"saveboard/internal/posts"
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
	authConfig := identity.AuthConfig{SigningKey: cfg.AuthSigningKey, EncryptionKey: cfg.AuthEncryptionKey, Issuer: cfg.PublicURL, Audience: "saveboard-api", OIDCIssuer: cfg.OIDCIssuer}
	if cfg.OIDCIssuer != "" {
		authConfig.ResolveProvider = identity.ProviderResolver(cfg.OIDCIssuer, cfg.OIDCClientID, cfg.OIDCClientSecret, cfg.PublicURL+"/auth/callback", cfg.OIDCBackchannelURL)
	}
	auth, err := identity.NewAuthService(database.DB, authConfig)
	if err != nil {
		return err
	}
	postService := posts.NewService(database.DB)
	if root := os.Getenv("BLOB_ROOT"); root != "" {
		postService.Blobs = blobstore.ReadOnly(root)
	}
	handler, _ := httpapi.New(httpapi.Dependencies{Ping: database.PingContext, Auth: auth, Posts: postService, Notifications: &notifications.Service{DB: database.DB}, SecureCookies: cfg.SecureCookies})
	return httpserver.Run(ctx, cfg.HTTPAddr, handler)
}
