package httpx

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey string

const UserIDContextKey contextKey = "userID"

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func ReadJSON(r *http.Request, dst any) error {
	return json.NewDecoder(r.Body).Decode(dst)
}

func Error(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDContextKey, userID)
}

func UserID(ctx context.Context) string {
	value, _ := ctx.Value(UserIDContextKey).(string)
	return value
}
