package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"auth-service/internal/token"
	"auth-service/internal/model"
	"auth-service/internal/store"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

var loginLimiter = rate.NewLimiter(rate.Limit(5), 10)

type AuthHandler struct {
	store store.Store
}

func NewAuthHandler(s store.Store) *AuthHandler {
	return &AuthHandler{store: s}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if !loginLimiter.Allow() {
		jsonError(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}

	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		jsonError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	var req LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		jsonError(w, "Missing username or password", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByUsername(req.Username)
	if err != nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		jsonError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	rawToken, err := token.GenerateToken(user.Role, user.Username)
	if err != nil {
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(LoginResponse{Token: rawToken})
}

type MeResponse struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (h *AuthHandler) MeHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(model.ClaimsKey).(*model.CustomClaims)
	if !ok || claims == nil {
		http.Error(w, "Unauthorized: Invalid or missing token claims", http.StatusUnauthorized)
		return
	}

	resp := MeResponse{
		Username: claims.Username,
		Role:     claims.Role,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

type PasswordChangeRequest struct {
	CurrentPassword string `json:"currentpassword"`
	NewPassword     string `json:"newpassword"`
}

func (h *AuthHandler) PasswordChangeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := r.Context().Value(model.ClaimsKey).(*model.CustomClaims)
	if !ok || claims == nil {
		http.Error(w, "Unauthorized: Invalid or missing token claims", http.StatusUnauthorized)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		jsonError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	defer r.Body.Close()
	var req PasswordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.CurrentPassword) == "" || strings.TrimSpace(req.NewPassword) == "" {
		jsonError(w, "Missing current or new password", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUserByUsername(claims.Username)
	if err != nil {
		jsonError(w, "User not found", http.StatusNotFound)
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword))
	if err != nil {
		jsonError(w, "Incorrect current password", http.StatusUnauthorized)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), getBcryptCost())
	if err != nil {
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = h.store.UpdateUser(user.ID, user.Username, string(newHash), user.Role)
	if err != nil {
		jsonError(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Password updated successfully"}`))
}
