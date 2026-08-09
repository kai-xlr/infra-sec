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

func TestDeleteSession(t *testing.T) {
	s := store.NewInMemoryStore()

	session, err := s.CreateSession(1, "admin", "admin", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if err := s.DeleteSession(session.ID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	if _, err := s.GetSessionByToken(session.Token); err == nil {
		t.Error("expected error for deleted session token, got nil")
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	s := store.NewInMemoryStore()

	err := s.DeleteSession(999)
	if err == nil {
		t.Fatal("expected error for nonexistent session, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got '%v'", err)
	}
}

func TestDeleteSessionsByUserID(t *testing.T) {
	s := store.NewInMemoryStore()

	keep, err := s.CreateSession(1, "admin", "admin", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	other1, err := s.CreateSession(1, "admin", "admin", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	other2, err := s.CreateSession(2, "viewer", "viewer", 15*time.Minute)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if err := s.DeleteSessionsByUserID(1, keep.Token); err != nil {
		t.Fatalf("DeleteSessionsByUserID failed: %v", err)
	}

	if _, err := s.GetSessionByToken(keep.Token); err != nil {
		t.Errorf("expected excluded session to remain, got error: %v", err)
	}
	if _, err := s.GetSessionByToken(other1.Token); err == nil {
		t.Error("expected other session for user 1 to be deleted, got nil error")
	}
	if _, err := s.GetSessionByToken(other2.Token); err != nil {
		t.Errorf("expected other user's session to remain, got error: %v", err)
	}
}

func TestListSessionsByUserID(t *testing.T) {
	s := store.NewInMemoryStore()

	if _, err := s.CreateSession(1, "admin", "admin", 15*time.Minute); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := s.CreateSession(1, "admin", "admin", 15*time.Minute); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, err := s.CreateSession(1, "admin", "admin", 15*time.Minute); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := s.CreateSession(2, "viewer", "viewer", 15*time.Minute); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	sessions, err := s.ListSessionsByUserID(1)
	if err != nil {
		t.Fatalf("ListSessionsByUserID failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}
	for i := 0; i < len(sessions)-1; i++ {
		if sessions[i].CreatedAt.Before(sessions[i+1].CreatedAt) {
			t.Errorf(
				"sessions not sorted by created_at desc: index %d (%v) before index %d (%v)",
				i, sessions[i].CreatedAt, i+1, sessions[i+1].CreatedAt,
			)
		}
	}
}

func TestListSessionsByUserIDExcludesExpired(t *testing.T) {
	s := store.NewInMemoryStore()

	if _, err := s.CreateSession(1, "admin", "admin", time.Millisecond); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := s.CreateSession(1, "admin", "admin", 15*time.Minute); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	sessions, err := s.ListSessionsByUserID(1)
	if err != nil {
		t.Fatalf("ListSessionsByUserID failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 non-expired session, got %d", len(sessions))
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	s := store.NewInMemoryStore()

	expired, err := s.CreateSession(1, "admin", "admin", time.Millisecond)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := s.CreateSession(1, "admin", "admin", time.Millisecond); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := s.CreateSession(1, "admin", "admin", 15*time.Minute); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := s.CreateSession(2, "viewer", "viewer", time.Millisecond); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	count, err := s.DeleteExpiredSessions()
	if err != nil {
		t.Fatalf("DeleteExpiredSessions failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 expired sessions deleted, got %d", count)
	}

	if _, err := s.GetSessionByToken(expired.Token); err == nil {
		t.Error("expected expired session to be removed, got nil error")
	}

	sessions, err := s.ListSessionsByUserID(1)
	if err != nil {
		t.Fatalf("ListSessionsByUserID failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 non-expired session to remain, got %d", len(sessions))
	}
	if sessions[0].Role != "admin" || sessions[0].Username != "admin" {
		t.Errorf("unexpected remaining session: %+v", sessions[0])
	}
}
