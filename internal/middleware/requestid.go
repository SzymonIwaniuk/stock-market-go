package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKey string

const RequestIDKey ctxKey = "request_id"
const RequestIDHeader = "X-Request-ID"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = generateID()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
