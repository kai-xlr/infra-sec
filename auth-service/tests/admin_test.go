package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"auth-service/internal/handler"
	"auth-service/internal/audit"
	"auth-service/internal/token"
	"auth-service/internal/middleware"
	"auth-service/internal/store"
)

func setupAdminTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	s := store.NewInMemoryStore()
	seedHash := "$2a$10$dv0AcULv0j9unVsTZvIxpeaGYLryIi17tEiiZp./dUm4Ab8fXQvqq"
	_, err := s.CreateUser("admin", seedHash, "admin")
	if err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}
	_, err = s.CreateUser("vieweruser", seedHash, "viewer")
	if err != nil {
		t.Fatalf("failed to seed viewer: %v", err)
	}

	auditFile, err := os.CreateTemp("", "audit-*.log")
	if err != nil {
		t.Fatalf("failed to create audit temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(auditFile.Name()) })

	auditLogger, err := audit.NewLogger(auditFile.Name())
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}

	authHandler := handler.NewAuthHandler(s)

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/admin/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.ListUsersHandler(w, r)
		case http.MethodPost:
			authHandler.CreateUserHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	adminMux.HandleFunc("/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.GetUserHandler(w, r)
		case http.MethodPut:
			authHandler.UpdateUserHandler(w, r)
		case http.MethodDelete:
			authHandler.DeleteUserHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	adminHandler := middleware.AuthMiddleware(
		middleware.RequireRole("admin", auditLogger)(adminMux),
	)

	mux := http.NewServeMux()
	mux.Handle("/admin/", adminHandler)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server
}

func adminRequest(t *testing.T, server *httptest.Server, method, path string, body []byte, token string) *http.Response {
	t.Helper()

	req, _ := http.NewRequest(method, server.URL+path, nil)
	if body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, path, err)
	}
	return resp
}

func TestCreateUserAsAdmin(t *testing.T) {
	server := setupAdminTestServer(t)

	adminToken, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	body := `{"username":"newuser","password":"pass123","role":"developer"}`
	resp := adminRequest(t, server, "POST", "/admin/users", []byte(body), adminToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["username"] != "newuser" {
		t.Errorf("expected username 'newuser', got '%v'", result["username"])
	}
	if result["role"] != "developer" {
		t.Errorf("expected role 'developer', got '%v'", result["role"])
	}
	if result["id"] == nil || result["id"].(float64) <= 0 {
		t.Errorf("expected positive id, got %v", result["id"])
	}
}

func TestCreateUserAsViewerForbidden(t *testing.T) {
	server := setupAdminTestServer(t)

	viewerToken, err := token.GenerateToken("viewer", "vieweruser")
	if err != nil {
		t.Fatalf("failed to generate viewer token: %v", err)
	}

	body := `{"username":"newuser","password":"pass123","role":"developer"}`
	resp := adminRequest(t, server, "POST", "/admin/users", []byte(body), viewerToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListUsers(t *testing.T) {
	server := setupAdminTestServer(t)

	adminToken, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	resp := adminRequest(t, server, "GET", "/admin/users", nil, adminToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var users []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&users)
	if len(users) < 2 {
		t.Errorf("expected at least 2 users, got %d", len(users))
	}
}

func TestGetUserByID(t *testing.T) {
	server := setupAdminTestServer(t)

	adminToken, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	resp := adminRequest(t, server, "GET", "/admin/users/1", nil, adminToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["username"] != "admin" {
		t.Errorf("expected username 'admin', got '%v'", result["username"])
	}
}

func TestUpdateUser(t *testing.T) {
	server := setupAdminTestServer(t)

	adminToken, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	createdAtBefore := ""
	{
		resp := adminRequest(t, server, "GET", "/admin/users/2", nil, adminToken)
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		createdAtBefore = fmt.Sprintf("%v", result["created_at"])
	}

	body := `{"username":"updateduser","role":"developer"}`
	resp := adminRequest(t, server, "PUT", "/admin/users/2", []byte(body), adminToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["username"] != "updateduser" {
		t.Errorf("expected username 'updateduser', got '%v'", result["username"])
	}
	if result["role"] != "developer" {
		t.Errorf("expected role 'developer', got '%v'", result["role"])
	}

	createdAtAfter := fmt.Sprintf("%v", result["created_at"])
	if createdAtBefore != createdAtAfter {
		t.Errorf("created_at should not change on update, before=%s after=%s", createdAtBefore, createdAtAfter)
	}
}

func TestDeleteUser(t *testing.T) {
	server := setupAdminTestServer(t)

	adminToken, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	resp := adminRequest(t, server, "DELETE", "/admin/users/2", nil, adminToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}

	resp = adminRequest(t, server, "GET", "/admin/users/2", nil, adminToken)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp.StatusCode)
	}
}

func TestCRUDLifecycle(t *testing.T) {
	server := setupAdminTestServer(t)

	adminToken, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	// Create
	body := `{"username":"lifecycle","password":"test123","role":"viewer"}`
	resp := adminRequest(t, server, "POST", "/admin/users", []byte(body), adminToken)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	userID := int(created["id"].(float64))

	// Get by ID
	resp = adminRequest(t, server, "GET", fmt.Sprintf("/admin/users/%d", userID), nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("get: expected 200, got %d", resp.StatusCode)
	}
	var fetched map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&fetched)
	resp.Body.Close()
	if fetched["username"] != "lifecycle" {
		t.Errorf("get: expected username 'lifecycle', got '%v'", fetched["username"])
	}

	// Update
	updateBody := `{"username":"lifecycle_updated","role":"developer"}`
	resp = adminRequest(t, server, "PUT", fmt.Sprintf("/admin/users/%d", userID), []byte(updateBody), adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("update: expected 200, got %d", resp.StatusCode)
	}
	var updated map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&updated)
	resp.Body.Close()
	if updated["username"] != "lifecycle_updated" {
		t.Errorf("update: expected 'lifecycle_updated', got '%v'", updated["username"])
	}

	// Get again — verify change
	resp = adminRequest(t, server, "GET", fmt.Sprintf("/admin/users/%d", userID), nil, adminToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("get after update: expected 200, got %d", resp.StatusCode)
	}
	var refetched map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&refetched)
	resp.Body.Close()
	if refetched["username"] != "lifecycle_updated" {
		t.Errorf("get after update: expected 'lifecycle_updated', got '%v'", refetched["username"])
	}

	// Delete
	resp = adminRequest(t, server, "DELETE", fmt.Sprintf("/admin/users/%d", userID), nil, adminToken)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete: expected 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Get after delete — 404
	resp = adminRequest(t, server, "GET", fmt.Sprintf("/admin/users/%d", userID), nil, adminToken)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("get after delete: expected 404, got %d", resp.StatusCode)
	}
}
