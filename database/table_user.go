package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const (
	createAuthTablesSQL = `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			profile JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS accounts (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON accounts(user_id);
	`

	dropAuthTablesSQL = `DROP TABLE IF EXISTS accounts, users CASCADE`
)

// User represents a user in the database
type User struct {
	ID        string
	Profile   map[string]string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Account represents an authentication account
type Account struct {
	ID        string
	UserID    string
	Type      string
	Payload   map[string]any
	CreatedAt time.Time
}

// EmailPasswordPayload is the payload for email/password authentication
type EmailPasswordPayload struct {
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
}

// CreateUser creates a new user
func (c *Client) CreateUser(id string, profile map[string]string) error {
	var profileJSON []byte
	var err error

	if profile != nil && len(profile) > 0 {
		profileJSON, err = json.Marshal(profile)
		if err != nil {
			return fmt.Errorf("failed to marshal profile: %w", err)
		}
	}

	insertSQL := `
		INSERT INTO users (id, profile)
		VALUES ($1, $2::jsonb)
	`

	_, err = c.db.Exec(insertSQL, id, profileJSON)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUser retrieves a user by ID
func (c *Client) GetUser(id string) (*User, error) {
	querySQL := `
		SELECT id, profile, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user User
	var profileJSON sql.NullString
	var createdAt, updatedAt time.Time

	err := c.db.QueryRow(querySQL, id).Scan(
		&user.ID,
		&profileJSON,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.CreatedAt = createdAt
	user.UpdatedAt = updatedAt

	if profileJSON.Valid && profileJSON.String != "" {
		if err := json.Unmarshal([]byte(profileJSON.String), &user.Profile); err != nil {
			return nil, fmt.Errorf("failed to unmarshal profile: %w", err)
		}
	} else {
		user.Profile = make(map[string]string)
	}

	return &user, nil
}

// IsGuest checks if a user is a guest (has no account)
func (c *Client) IsGuest(userID string) (bool, error) {
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM accounts WHERE user_id = $1", userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check guest status: %w", err)
	}
	return count == 0, nil
}

// GetAccountByUserID retrieves an account by user ID
func (c *Client) GetAccountByUserID(userID string) (*Account, error) {
	querySQL := `
		SELECT id, user_id, type, payload, created_at
		FROM accounts
		WHERE user_id = $1
	`

	var account Account
	var payloadJSON sql.NullString
	var createdAt time.Time

	err := c.db.QueryRow(querySQL, userID).Scan(
		&account.ID,
		&account.UserID,
		&account.Type,
		&payloadJSON,
		&createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account: %w", err)
	}

	account.CreatedAt = createdAt

	if payloadJSON.Valid && payloadJSON.String != "" {
		if err := json.Unmarshal([]byte(payloadJSON.String), &account.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	} else {
		account.Payload = make(map[string]any)
	}

	return &account, nil
}

// GetAccountByEmail retrieves an account by email (for email_password type)
func (c *Client) GetAccountByEmail(email string) (*Account, error) {
	querySQL := `
		SELECT id, user_id, type, payload, created_at
		FROM accounts
		WHERE type = 'email_password' AND payload->>'email' = $1
	`

	var account Account
	var payloadJSON sql.NullString
	var createdAt time.Time

	err := c.db.QueryRow(querySQL, email).Scan(
		&account.ID,
		&account.UserID,
		&account.Type,
		&payloadJSON,
		&createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get account by email: %w", err)
	}

	account.CreatedAt = createdAt

	if payloadJSON.Valid && payloadJSON.String != "" {
		if err := json.Unmarshal([]byte(payloadJSON.String), &account.Payload); err != nil {
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	} else {
		account.Payload = make(map[string]any)
	}

	return &account, nil
}

// CreateAccount creates a new account for a user
func (c *Client) CreateAccount(id, userID, accountType string, payload map[string]any) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	insertSQL := `
		INSERT INTO accounts (id, user_id, type, payload)
		VALUES ($1, $2, $3, $4::jsonb)
	`

	_, err = c.db.Exec(insertSQL, id, userID, accountType, payloadJSON)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	return nil
}

// UpdateUserProfile updates the user's profile fields
func (c *Client) UpdateUserProfile(userID string, profile map[string]string) error {
	// Get existing profile
	user, err := c.GetUser(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// Merge with new profile fields
	if user.Profile == nil {
		user.Profile = make(map[string]string)
	}
	for key, value := range profile {
		user.Profile[key] = value
	}

	// Marshal updated profile
	profileJSON, err := json.Marshal(user.Profile)
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	// Update in database
	updateSQL := `
		UPDATE users
		SET profile = $1::jsonb, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	_, err = c.db.Exec(updateSQL, profileJSON, userID)
	if err != nil {
		return fmt.Errorf("failed to update user profile: %w", err)
	}

	return nil
}
