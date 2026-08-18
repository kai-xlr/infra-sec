package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"auth-service/internal/model"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

var _ Store = (*SQLiteStore)(nil)

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
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			username TEXT NOT NULL,
			role TEXT NOT NULL,
			token TEXT UNIQUE NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) CreateUser(username, passwordHash, role string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()

	query := `
	INSERT INTO users (username, password_hash, role, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?);`

	result, err := s.db.Exec(
		query,
		username,
		passwordHash,
		role,
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
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

	return &model.User{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *SQLiteStore) GetUser(id int64) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, username, password_hash, role, created_at, updated_at
	FROM users 
	WHERE id = ?;`

	var user model.User
	var createdAt string
	var updatedAt string

	err := s.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&createdAt,
		&updatedAt,
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

	user.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return &user, nil
}

func (s *SQLiteStore) GetUserByUsername(username string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, username, password_hash, role, created_at, updated_at
	FROM users 
	WHERE username = ?;`

	var user model.User
	var createdAt string
	var updatedAt string

	err := s.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&createdAt,
		&updatedAt,
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

	user.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
	}

	return &user, nil
}

func (s *SQLiteStore) ListUsers(role string) ([]*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, username, password_hash, role, created_at, updated_at
	FROM users`
	var args []interface{}
	if role != "" {
		query += " WHERE role = ?;"
		args = append(args, role)
	} else {
		query += ";"
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	users := make([]*model.User, 0)

	for rows.Next() {
		var user model.User
		var createdAt string
		var updatedAt string

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Role,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user row: %w", err)
		}

		user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at for user %d: %w", user.ID, err)
		}

		user.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated_at for user %d: %w", user.ID, err)
		}

		users = append(users, &user)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	return users, nil
}

func (s *SQLiteStore) UpdateUser(id int64, username, passwordHash, role string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	query := `
	UPDATE users
	SET username = ?, password_hash = ?, role = ?, updated_at = ?
	WHERE id = ?;`

	result, err := s.db.Exec(query, username, passwordHash, role, now.Format(time.RFC3339), id)
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

	var user model.User
	var createdAt string
	var updatedAt string

	selectQuery := `
	SELECT id, username, password_hash, role, created_at, updated_at
	FROM users
	WHERE id = ?;`

	err = s.db.QueryRow(selectQuery, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated user: %w", err)
	}

	user.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	user.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse updated_at: %w", err)
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

func (s *SQLiteStore) CreateSession(userID int64, username, role string, ttl time.Duration) (*model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	now := time.Now().UTC()
	token := hex.EncodeToString(b)
	expiresAt := now.Add(ttl)

	query := `
	INSERT INTO sessions (user_id, username, role, token, created_at, expires_at)
	VALUES (?, ?, ?, ?, ?, ?);`

	result, err := s.db.Exec(
		query,
		userID,
		username,
		role,
		token,
		now.Format(time.RFC3339),
		expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve last insert id: %w", err)
	}

	return &model.Session{
		ID:        id,
		UserID:    userID,
		Username:  username,
		Role:      role,
		Token:     token,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *SQLiteStore) GetSessionByToken(token string) (*model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, user_id, username, role, token, created_at, expires_at
	FROM sessions
	WHERE token = ?;`

	var session model.Session
	var createdAt string
	var expiresAt string

	err := s.db.QueryRow(query, token).Scan(
		&session.ID,
		&session.UserID,
		&session.Username,
		&session.Role,
		&session.Token,
		&createdAt,
		&expiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to fetch session by token: %w", err)
	}

	session.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at: %w", err)
	}

	session.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expires_at: %w", err)
	}

	return &session, nil
}

func (s *SQLiteStore) DeleteSession(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM sessions WHERE id = ?;`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to retrieve rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session with ID %d not found for deletion", id)
	}

	return nil
}

func (s *SQLiteStore) DeleteSessionsByUserID(userID int64, excludeToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM sessions WHERE user_id = ? AND token != ?;`

	_, err := s.db.Exec(query, userID, excludeToken)
	if err != nil {
		return fmt.Errorf("failed to delete sessions for user %d: %w", userID, err)
	}

	return nil
}

func (s *SQLiteStore) ListSessionsByUserID(userID int64) ([]*model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	query := `
	SELECT id, user_id, username, role, token, created_at, expires_at
	FROM sessions
	WHERE user_id = ? AND expires_at > ?
	ORDER BY created_at DESC;`

	rows, err := s.db.Query(query, userID, now.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions for user %d: %w", userID, err)
	}
	defer rows.Close()

	sessions := make([]*model.Session, 0)

	for rows.Next() {
		var session model.Session
		var createdAt string
		var expiresAt string

		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.Username,
			&session.Role,
			&session.Token,
			&createdAt,
			&expiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}

		session.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at for session %d: %w", session.ID, err)
		}

		session.ExpiresAt, err = time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse expires_at for session %d: %w", session.ID, err)
		}

		sessions = append(sessions, &session)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during session row iteration: %w", err)
	}

	return sessions, nil
}

func (s *SQLiteStore) DeleteExpiredSessions() (int64, error) {
	panic("not implemented")
}
