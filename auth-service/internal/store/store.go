package store

import "auth-service/internal/models"

type Store interface {
	CreateUser(username, passwordHash, role string) (*models.User, error)
	GetUser(id int64) (*models.User, error)
	GetUserByUsername(username string) (*models.User, error)
	ListUsers() ([]*models.User, error)
	UpdateUser(id int64, username, passwordHash, role string) (*models.User, error)
	DeleteUser(id int64) error
}
