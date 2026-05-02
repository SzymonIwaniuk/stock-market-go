package handler

import (
	"log/slog"
	"net/http"
	"os"
)

func Chaos(w http.ResponseWriter, r *http.Request) {
	slog.Warn("chaos endpoint hit, shutting down")
	w.WriteHeader(http.StatusOK)

	// Flush the response before exiting. We hijack the flusher so the
	// client actually receives the 200 before the process dies.
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		os.Exit(1)
	}()
}
