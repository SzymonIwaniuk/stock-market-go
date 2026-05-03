package handler

import (
	"encoding/json"
	"net/http"

	"github.com/szymoniwaniuk/stock-market-go/internal/model"
)

func writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.ErrResponse{
		HTTPStatusCode: status,
		Message:        msg,
	})
}
