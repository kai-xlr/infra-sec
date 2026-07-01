package store

import (
	"auth-service/internal/model"
	"fmt"
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
}

type InMemoryStore struct {
	mu     sync.RWMutex
	users  map[int64]*model.User
	nextID int64
}

var _ Store = (*InMemoryStore)(nil)

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		users:  make(map[int64]*model.User),
		nextID: 1,
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
