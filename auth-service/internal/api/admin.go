package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"auth-service/internal/models"

	"golang.org/x/crypto/bcrypt"
)

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type UpdateUserRequest struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
	Role     *string `json:"role"`
}

type UserResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func getBcryptCost() int {
	cost := bcrypt.DefaultCost
	value := os.Getenv("BCRYPT_COST")
	if value == "" {
		return cost
	}

	parsedCost, err := strconv.Atoi(value)
	if err != nil {
		return cost
	}

	return parsedCost
}

func (h *AuthHandler) ListUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	roleFilter := r.URL.Query().Get("role")

	users, err := h.store.ListUsers(roleFilter)
	if err != nil {
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if users == nil {
		users = []*models.User{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *AuthHandler) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
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

	var req CreateUserRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" || strings.TrimSpace(req.Role) == "" {
		jsonError(w, "Username, password, and role are required", http.StatusBadRequest)
		return
	}

	validRoles := map[string]bool{
		"admin":     true,
		"developer": true,
		"viewer":    true,
	}

	if !validRoles[req.Role] {
		jsonError(
			w,
			"Invalid role: must be admin, developer, or viewer",
			http.StatusBadRequest,
		)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), getBcryptCost())
	if err != nil {
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	user, err := h.store.CreateUser(req.Username, string(hashedPassword), req.Role)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *AuthHandler) GetUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		jsonError(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	user, err := h.store.GetUser(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AuthHandler) UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	idStr := r.PathValue("id")
	if idStr == "" {
		jsonError(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	existingUser, err := h.store.GetUser(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	updatedUsername := existingUser.Username
	updatedPasswordHash := existingUser.PasswordHash
	updatedRole := existingUser.Role

	if req.Username != nil {
		updatedUsername = *req.Username
	}
	if req.Role != nil {
		validRoles := map[string]bool{
			"admin":     true,
			"developer": true,
			"viewer":    true,
		}

		if !validRoles[*req.Role] {
			jsonError(
				w,
				"Invalid role: must be admin, developer, or viewer",
				http.StatusBadRequest,
			)
			return
		}

		updatedRole = *req.Role
	}
	if req.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), getBcryptCost())
		if err != nil {
			jsonError(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		updatedPasswordHash = string(hashedPassword)
	}

	user, err := h.store.UpdateUser(id, updatedUsername, updatedPasswordHash, updatedRole)
	if err != nil {
		if strings.Contains(err.Error(), "already taken") {
			jsonError(w, err.Error(), http.StatusConflict)
			return
		}
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	resp := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AuthHandler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		jsonError(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		jsonError(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	err = h.store.DeleteUser(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		jsonError(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
