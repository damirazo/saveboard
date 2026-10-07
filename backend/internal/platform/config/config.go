package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
	ServiceName string
}

func Load(service string) (Config, error) {
	c := Config{HTTPAddr: env("HTTP_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), ServiceName: service}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return c, fmt.Errorf("DATABASE_URL must be a PostgreSQL connection URL")
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
