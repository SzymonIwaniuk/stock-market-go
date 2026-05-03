package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/szymoniwaniuk/stock-market-go/internal/api"
	"github.com/szymoniwaniuk/stock-market-go/internal/middleware"
	"github.com/szymoniwaniuk/stock-market-go/internal/store"
)

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	redisAddr := flag.String("redis", "localhost:6379", "Redis address")
	logFormat := flag.String("log-format", "json", "Log format: json or text")
	flag.Parse()

	middleware.SetupLogger(middleware.LoggerConfig{
		Level:  slog.LevelInfo,
		Format: *logFormat,
	})

	slog.Info("starting stock market service", "port", *port, "redis", *redisAddr)

	redisStore := store.NewRedisStore(*redisAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := redisStore.Ping(ctx); err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to redis")

	a := api.New(*port, redisStore)

	go a.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
