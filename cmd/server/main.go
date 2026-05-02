package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/szymoniwaniuk/stock-market-go/internal/handler"
	"github.com/szymoniwaniuk/stock-market-go/internal/middleware"
	"github.com/szymoniwaniuk/stock-market-go/internal/store"
)

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	redisAddr := flag.String("redis", "localhost:6379", "Redis address")
	flag.Parse()

	slog.Info("starting stock market service", "port", *port, "redis", *redisAddr)

	redisStore := store.NewRedisStore(*redisAddr)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := redisStore.Ping(ctx); err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	slog.Info("connected to redis")

	walletH := handler.NewWalletHandler(redisStore)
	stockH := handler.NewStockHandler(redisStore)
	logH := handler.NewLogHandler(redisStore)

	r := chi.NewRouter()
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestLogger)

	r.Post("/wallets/{wallet_id}/stocks/{stock_name}", walletH.Trade)
	r.Get("/wallets/{wallet_id}", walletH.GetWallet)
	r.Get("/wallets/{wallet_id}/stocks/{stock_name}", walletH.GetWalletStock)

	r.Get("/stocks", stockH.GetStocks)
	r.Post("/stocks", stockH.SetStocks)

	r.Get("/log", logH.GetLog)

	r.Post("/chaos", handler.Chaos)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", *port),
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down gracefully")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
