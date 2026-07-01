package handler

import (
	"encoding/json"
	"net/http"

	"auth-service/internal/model"
)

func WhoamiHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(model.ClaimsKey).(*model.CustomClaims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	resp := map[string]string{
		"subject": claims.Subject,
		"role":    claims.Role,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
