package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"sudoku-backend/internal/app"
	"sudoku-backend/internal/config"
)

func main() {
	cfg := config.Load()

	server, err := app.NewServer(cfg)
	if err != nil {
		log.Fatalf("bootstrap server: %v", err)
	}
	defer server.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("backend listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server error: %v", err)
	}
}
