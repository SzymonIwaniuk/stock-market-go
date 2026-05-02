package handler

import (
	"encoding/json"
	"net/http"

	"github.com/szymoniwaniuk/stock-market-go/internal/model"
)

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.ErrResponse{
		HTTPStatusCode: status,
		Message:        msg,
	})
}
