package store

import (
	"auth-service/internal/model"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

type Store interface {
	CreateUser(username, passwordHash, role string) (*model.User, error)
	GetUser(id int64) (*model.User, error)
	GetUserByUsername(username string) (*model.User, error)
	ListUsers(role string) ([]*model.User, error)
	UpdateUser(id int64, username, passwordHash, role string) (*model.User, error)
	DeleteUser(id int64) error
	CreateSession(userID int64, username, role string, ttl time.Duration) (*model.Session, error)
	GetSessionByToken(token string) (*model.Session, error)
	GetSessionByID(id int64) (*model.Session, error)
	DeleteSession(id int64) error
	DeleteSessionsByUserID(userID int64, excludeToken string) error
	ListSessionsByUserID(userID int64) ([]*model.Session, error)
	DeleteExpiredSessions() (int64, error)
}

type InMemoryStore struct {
	mu            sync.RWMutex
	users         map[int64]*model.User
	nextID        int64
	sessions      map[int64]*model.Session
	nextSessionID int64
}

var _ Store = (*InMemoryStore)(nil)

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		users:         make(map[int64]*model.User),
		nextID:        1,
		sessions:      make(map[int64]*model.Session),
		nextSessionID: 1,
	}
}

func (s *InMemoryStore) CreateUser(username, passwordHash, role string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.users {
		if u.Username == username {
			return nil, fmt.Errorf("user with username '%s' already exists", username)
		}
	}

	now := time.Now().UTC()
	user := &model.User{
		ID:           s.nextID,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	s.users[s.nextID] = user
	s.nextID++

	return user, nil
}

func (s *InMemoryStore) GetUser(id int64) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", id)
	}
	return user, nil
}

func (s *InMemoryStore) GetUserByUsername(username string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user with username '%s' not found", username)
}

func (s *InMemoryStore) ListUsers(role string) ([]*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*model.User, 0, len(s.users))
	for _, u := range s.users {
		if role == "" || u.Role == role {
			users = append(users, u)
		}
	}
	return users, nil
}

func (s *InMemoryStore) UpdateUser(
	id int64,
	username, passwordHash, role string,
) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found for update", id)
	}

	for _, u := range s.users {
		if u.Username == username && u.ID != id {
			return nil, fmt.Errorf("username '%s' is already taken by another user", username)
		}
	}

	user.Username = username
	user.PasswordHash = passwordHash
	user.Role = role
	user.UpdatedAt = time.Now().UTC()

	return user, nil
}

func (s *InMemoryStore) DeleteUser(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.users[id]; !exists {
		return fmt.Errorf("user with ID %d not found for deletion", id)
	}

	delete(s.users, id)
	return nil
}

func (s *InMemoryStore) CreateSession(
	userID int64,
	username, role string,
	ttl time.Duration,
) (*model.Session, error) {
	if ttl <= 0 {
		return nil, fmt.Errorf("session TTL must be positive, got %s", ttl)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	now := time.Now().UTC()
	session := &model.Session{
		ID:        s.nextSessionID,
		UserID:    userID,
		Username:  username,
		Role:      role,
		Token:     hex.EncodeToString(b),
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	s.sessions[s.nextSessionID] = session
	s.nextSessionID++

	return session, nil
}

func (s *InMemoryStore) GetSessionByToken(token string) (*model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if session.Token == token {
			return session, nil
		}
	}
	return nil, fmt.Errorf("session not found")
}

func (s *InMemoryStore) GetSessionByID(id int64) (*model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session with ID %d not found", id)
	}
	return session, nil
}

func (s *InMemoryStore) DeleteSession(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[id]; !exists {
		return fmt.Errorf("session with ID %d not found for deletion", id)
	}

	delete(s.sessions, id)
	return nil
}

func (s *InMemoryStore) DeleteSessionsByUserID(userID int64, excludeToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, session := range s.sessions {
		if session.UserID == userID && session.Token != excludeToken {
			delete(s.sessions, id)
		}
	}
	return nil
}

func (s *InMemoryStore) ListSessionsByUserID(userID int64) ([]*model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	sessions := make([]*model.Session, 0)
	for _, session := range s.sessions {
		if session.UserID == userID && session.ExpiresAt.After(now) {
			sessions = append(sessions, session)
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})

	return sessions, nil
}

func (s *InMemoryStore) DeleteExpiredSessions() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	var count int64

	for id, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, id)
			count++
		}
	}

	return count, nil
}
