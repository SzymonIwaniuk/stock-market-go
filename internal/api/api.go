package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/szymoniwaniuk/stock-market-go/internal/handler"
	"github.com/szymoniwaniuk/stock-market-go/internal/middleware"
	"github.com/szymoniwaniuk/stock-market-go/internal/store"
)

type Api struct {
	Port   string
	Store  store.Store
	Router *chi.Mux
	server *http.Server
}

func New(port string, s store.Store) *Api {
	a := &Api{
		Port:  port,
		Store: s,
	}
	a.initRouter()
	return a
}

func (a *Api) initRouter() {
	a.Router = chi.NewRouter()
	a.Router.Use(middleware.RequestID)
	a.Router.Use(chimw.Recoverer)
	a.Router.Use(middleware.RequestLogger)
	a.Router.Use(middleware.ContentTypeJSON)

	walletH := handler.NewWalletHandler(a.Store)
	stockH := handler.NewStockHandler(a.Store)
	logH := handler.NewLogHandler(a.Store)
	resetH := handler.NewResetHandler(a.Store)

	a.Router.Route("/wallets/{wallet_id}", func(r chi.Router) {
		r.Get("/", walletH.GetWallet)
		r.Route("/stocks/{stock_name}", func(r chi.Router) {
			r.Post("/", walletH.Trade)
			r.Get("/", walletH.GetWalletStock)
		})
	})

	a.Router.Route("/stocks", func(r chi.Router) {
		r.Get("/", stockH.GetStocks)
		r.Post("/", stockH.SetStocks)
	})

	a.Router.Get("/log", logH.GetLog)
	a.Router.Post("/chaos", handler.Chaos)
	a.Router.Get("/health", handler.Health)
	a.Router.Post("/reset", resetH.Reset)
}

func (a *Api) Start() {
	a.server = &http.Server{
		Addr:         fmt.Sprintf(":%s", a.Port),
		Handler:      a.Router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("listening", "addr", a.server.Addr)
	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
	}
}

func (a *Api) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}
