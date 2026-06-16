package store

import (
	"auth-service/internal/models"
	"fmt"
	"sync"
	"time"
)

type Store interface {
	CreateUser(username, passwordHash, role string) (*models.User, error)
	GetUser(id int64) (*models.User, error)
	GetUserByUsername(username string) (*models.User, error)
	ListUsers() ([]*models.User, error)
	UpdateUser(id int64, username, passwordHash, role string) (*models.User, error)
	DeleteUser(id int64) error
}

type InMemoryStore struct {
	mu     sync.RWMutex
	users  map[int64]*models.User
	nextID int64
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		users:  make(map[int64]*models.User),
		nextID: 1,
	}
}

func (s *InMemoryStore) CreateUser(username, passwordHash, role string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.users {
		if u.Username == username {
			return nil, fmt.Errorf("user with username '%s' already exists", username)
		}
	}

	user := &models.User{
		ID:           s.nextID,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    time.Now().UTC(),
	}

	s.users[s.nextID] = user
	s.nextID++

	return user, nil
}
func (s *InMemoryStore) GetUser(id int64) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return nil, fmt.Errorf("user with ID %d not found", id)
	}
	return user, nil
}
func (s *InMemoryStore) GetUserByUsername(username string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user with username '%s' not found", username)
}
func (s *InMemoryStore) ListUsers() ([]*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]*models.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users, nil
}

func (s *InMemoryStore) UpdateUser(
	id int64,
	username, passwordHash, role string,
) (*models.User, error) {
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
