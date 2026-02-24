package database

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Client implements database operations
type Client struct {
	db *sql.DB
}

// Config holds configuration for the database client
type Config struct {
	DatabaseURL string
}

// NewClient creates a new database client
func NewClient(cfg Config) (*Client, error) {
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	client := &Client{db: db}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	return client, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	return c.db.Close()
}
