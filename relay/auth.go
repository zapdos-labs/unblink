package relay

import (
	"database/sql"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// AuthStore handles authentication operations
type AuthStore struct {
	db *sql.DB
}

// NewAuthStore creates a new AuthStore
func NewAuthStore(db *sql.DB) *AuthStore {
	return &AuthStore{db: db}
}

// Register creates a new user with email, password, and name
func (s *AuthStore) Register(email, password, name string) (*User, error) {
	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Insert user
	result, err := s.db.Exec(
		"INSERT INTO users (email, password_hash, name) VALUES (?, ?, ?)",
		email, string(hash), name,
	)
	if err != nil {
		return nil, err
	}

	// Get the user ID
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &User{
		ID:    id,
		Email: email,
		Name:  name,
	}, nil
}

// Login authenticates a user with email and password
func (s *AuthStore) Login(email, password string) (*User, error) {
	var user User
	var passwordHash string

	// Query user by email
	err := s.db.QueryRow(
		"SELECT id, email, password_hash, name FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &passwordHash, &user.Name)

	if err == sql.ErrNoRows {
		return nil, errors.New("invalid credentials")
	}
	if err != nil {
		return nil, err
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	return &user, nil
}

// GetUserByID retrieves a user by ID
func (s *AuthStore) GetUserByID(id int64) (*User, error) {
	var user User
	err := s.db.QueryRow(
		"SELECT id, email, name FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Email, &user.Name)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}
