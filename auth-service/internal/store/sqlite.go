package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"auth-service/internal/models"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	store := &SQLiteStore{
		db: db,
	}

	if err := store.createTables(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL,
		created_at TEXT NOT NULL
	);`

	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create user table: %w", err)
	}

	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) CreateUser(username, passwordHash, role string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	createdAt := time.Now().UTC()

	query := `
	INSERT INTO users (username, password_hash, role, created_at)
	VALUES (?, ?, ?, ?);`

	result, err := s.db.Exec(
		query,
		username,
		passwordHash,
		role,
		createdAt.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("username '%s' already exists", username)
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve last insert id: %w", err)
	}

	return &models.User{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    createdAt,
	}, nil
}

func (s *SQLiteStore) GetUser(id int64) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, username, password_hash, role, created_at 
	FROM users 
	WHERE id = ?;`

	var user models.User
	var createdAt string

	err := s.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user with id %d not found", id)
		}
		return nil, fmt.Errorf("failed to fetch user by id: %w", err)
	}

	user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	return &user, nil
}

func (s *SQLiteStore) GetUserByUsername(username string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, username, password_hash, role, created_at 
	FROM users 
	WHERE username = ?;`

	var user models.User
	var createdAt string

	err := s.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user with username '%s' not found", username)
		}
		return nil, fmt.Errorf("failed to fetch user by username: %w", err)
	}

	user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	return &user, nil
}

func (s *SQLiteStore) ListUsers() ([]*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, username, password_hash, role, created_at
	FROM users;`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	users := make([]*models.User, 0)

	for rows.Next() {
		var user models.User
		var createdAt string

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Role,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}

		user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at for user %d: %w", user.ID, err)
		}

		users = append(users, &user)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return users, nil
}

func (s *SQLiteStore) UpdateUser(id int64, username, passwordHash, role string) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	UPDATE users
	SET username = ?, password_hash = ?, role = ?
	WHERE id = ?;`

	result, err := s.db.Exec(query, username, passwordHash, role, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("username '%s' is already taken", username)
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return nil, fmt.Errorf("user with id %d not found", id)
	}

	var user models.User
	var createdAt string

	selectQuery := `
	SELECT id, username, password_hash, role, created_at
	FROM users
	WHERE id = ?;`

	err = s.db.QueryRow(selectQuery, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated user: %w", err)
	}

	user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	return &user, nil
}

func (s *SQLiteStore) DeleteUser(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM users WHERE id = ?;`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to execute delete query: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user with id %d not found", id)
	}

	return nil
}
