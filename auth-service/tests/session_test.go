package tests

import (
	"strings"
	"testing"
	"time"

	"auth-service/internal/store"
)

func TestCreateSession(t *testing.T) {
	s := store.NewInMemoryStore()

	now := time.Now().UTC()
	session, err := s.CreateSession(1, "admin", "admin", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID != 1 {
		t.Errorf("expected ID 1, got %d", session.ID)
	}
	if session.UserID != 1 {
		t.Errorf("expected UserID 1, got %d", session.UserID)
	}
	if session.Username != "admin" {
		t.Errorf("expected username 'admin', got '%s'", session.Username)
	}
	if session.Role != "admin" {
		t.Errorf("expected role 'admin', got '%s'", session.Role)
	}
	if len(session.Token) != 64 {
		t.Errorf("expected 64-char token, got %d chars", len(session.Token))
	}
	if session.CreatedAt.Before(now.Add(-time.Minute)) || session.CreatedAt.After(time.Now().UTC().Add(time.Minute)) {
		t.Errorf("CreatedAt %v not near now", session.CreatedAt)
	}
	wantExpiry := session.CreatedAt.Add(15 * time.Minute)
	if !session.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("expected ExpiresAt %v, got %v", wantExpiry, session.ExpiresAt)
	}
}

func TestCreateSessionUniqueTokens(t *testing.T) {
	s := store.NewInMemoryStore()

	s1, err := s.CreateSession(1, "admin", "admin", 15*time.Minute)
	if err != nil {
		t.Fatalf("first CreateSession failed: %v", err)
	}
	s2, err := s.CreateSession(1, "admin", "admin", 15*time.Minute)
	if err != nil {
		t.Fatalf("second CreateSession failed: %v", err)
	}

	if s1.Token == s2.Token {
		t.Errorf("expected unique tokens, got duplicate '%s'", s1.Token)
	}
	if s1.ID == s2.ID {
		t.Errorf("expected unique IDs, both %d", s1.ID)
	}
}

func TestCreateSessionInvalidTTL(t *testing.T) {
	s := store.NewInMemoryStore()

	for _, ttl := range []time.Duration{0, -time.Minute} {
		if _, err := s.CreateSession(1, "admin", "admin", ttl); err == nil {
			t.Errorf("expected error for TTL %s, got nil", ttl)
		}
	}
}

func TestGetSessionByToken(t *testing.T) {
	s := store.NewInMemoryStore()

	session, err := s.CreateSession(1, "admin", "admin", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	found, err := s.GetSessionByToken(session.Token)
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
	}
	if found.ID != session.ID {
		t.Errorf("expected ID %d, got %d", session.ID, found.ID)
	}
	if found.Token != session.Token {
		t.Errorf("expected token '%s', got '%s'", session.Token, found.Token)
	}
	if found.Username != session.Username {
		t.Errorf("expected username '%s', got '%s'", session.Username, found.Username)
	}
}

func TestGetSessionByTokenNotFound(t *testing.T) {
	s := store.NewInMemoryStore()

	if _, err := s.CreateSession(1, "admin", "admin", 15*time.Minute); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	_, err := s.GetSessionByToken("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent token, got nil")
	}
	if !strings.Contains(err.Error(), "session not found") {
		t.Errorf("expected 'session not found' in error, got '%v'", err)
	}
}
