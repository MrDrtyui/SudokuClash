package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	RedisAddr           string
	RedisPassword       string
	JWTSecret           string
	FrontendURL         string
	StripeSecretKey     string
	StripeWebhookSecret string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	ShutdownTimeout     time.Duration
	MatchmakingWindow   time.Duration
}

func Load() Config {
	return Config{
		HTTPAddr:            getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgresql://postgres:changeme@localhost:5432/appdb?sslmode=disable"),
		RedisAddr:           getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:       os.Getenv("REDIS_PASSWORD"),
		JWTSecret:           getEnv("JWT_SECRET", "dev-secret-change-me"),
		FrontendURL:         getEnv("FRONTEND_URL", "http://127.0.0.1:5173"),
		StripeSecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		AccessTokenTTL:      getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:     getDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		ShutdownTimeout:     getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		MatchmakingWindow:   getDuration("MATCHMAKING_WINDOW", 45*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
