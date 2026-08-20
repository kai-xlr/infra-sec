package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"auth-service/internal/handler"
	"auth-service/internal/token"
	"auth-service/internal/middleware"
	"auth-service/internal/store"
)

func setupTestServer(t *testing.T) (*httptest.Server, *store.InMemoryStore) {
	t.Helper()

	s := store.NewInMemoryStore()
	seedHash := "$2a$10$dv0AcULv0j9unVsTZvIxpeaGYLryIi17tEiiZp./dUm4Ab8fXQvqq"
	_, err := s.CreateUser("admin", seedHash, "admin")
	if err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}

	authHandler := handler.NewAuthHandler(s)

	protected := http.NewServeMux()
	protected.HandleFunc("/whoami", handler.WhoamiHandler)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.HealthHandler)
	mux.HandleFunc("/auth/login", authHandler.LoginHandler)
	mux.Handle("/whoami", middleware.AuthMiddleware(protected))
	mux.Handle("/auth/sessions", middleware.AuthMiddleware(http.HandlerFunc(authHandler.SessionsHandler)))

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server, s
}

func login(t *testing.T, server *httptest.Server, username, password string) (int, string, string) {
	t.Helper()

	body := `{"username":"` + username + `","password":"` + password + `"}`
	resp, err := http.Post(server.URL+"/auth/login", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	return resp.StatusCode, result["token"], result["session_token"]
}

func TestHealthEndpoint(t *testing.T) {
	server, _ := setupTestServer(t)

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLoginValidCredentials(t *testing.T) {
	server, _ := setupTestServer(t)

	code, token, sessionToken := login(t, server, "admin", "admin123")
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if sessionToken == "" {
		t.Error("expected non-empty session_token")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	server, _ := setupTestServer(t)

	code, _, _ := login(t, server, "admin", "wrongpassword")
	if code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestLoginMissingFields(t *testing.T) {
	server, _ := setupTestServer(t)

	tests := []struct {
		name string
		body string
	}{
		{"empty username", `{"username":"","password":"admin123"}`},
		{"empty password", `{"username":"admin","password":""}`},
		{"missing username", `{"password":"admin123"}`},
		{"missing password", `{"username":"admin"}`},
		{"empty object", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(server.URL+"/auth/login", "application/json", strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestWhoamiWithoutToken(t *testing.T) {
	server, _ := setupTestServer(t)

	resp, err := http.Get(server.URL + "/whoami")
	if err != nil {
		t.Fatalf("whoami request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWhoamiWithValidToken(t *testing.T) {
	server, _ := setupTestServer(t)

	token, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req, _ := http.NewRequest("GET", server.URL+"/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("whoami request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["subject"] != "admin" {
		t.Errorf("expected subject 'admin', got '%s'", result["subject"])
	}
	if result["role"] != "admin" {
		t.Errorf("expected role 'admin', got '%s'", result["role"])
	}
}

func TestSessionsWithoutToken(t *testing.T) {
	server, _ := setupTestServer(t)

	resp, err := http.Get(server.URL + "/auth/sessions")
	if err != nil {
		t.Fatalf("sessions request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSessionsWithValidToken(t *testing.T) {
	server, s := setupTestServer(t)

	user, err := s.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	_, err = s.CreateSession(user.ID, user.Username, user.Role, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	jwtToken, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req, _ := http.NewRequest("GET", server.URL+"/auth/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sessions request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var sessions []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&sessions)
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0]["token"] != nil {
		t.Error("session response should not include token field")
	}
}

func TestSessionsEmptyWhenNoSessions(t *testing.T) {
	server, _ := setupTestServer(t)

	jwtToken, err := token.GenerateToken("admin", "admin")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req, _ := http.NewRequest("GET", server.URL+"/auth/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+jwtToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sessions request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var sessions []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&sessions)
	if len(sessions) != 0 {
		t.Errorf("expected empty array, got %d sessions", len(sessions))
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
