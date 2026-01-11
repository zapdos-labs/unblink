package relay

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/tursodatabase/turso-go"
)

// Database wraps sql.DB with Turso-specific functionality
type Database struct {
	*sql.DB
}

// NewDatabase creates a new database connection and initializes the schema
func NewDatabase(dbPath string) (*Database, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("turso", dbPath)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Initialize schema
	if err := initSchema(db); err != nil {
		return nil, err
	}

	return &Database{db}, nil
}

// initSchema creates the necessary tables if they don't exist
func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

	CREATE TABLE IF NOT EXISTS nodes (
		id TEXT PRIMARY KEY,
		token TEXT UNIQUE NOT NULL,
		owner_id INTEGER,
		name TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		authorized_at DATETIME,
		last_connected_at DATETIME,
		FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE SET NULL
	);
	CREATE INDEX IF NOT EXISTS idx_nodes_token ON nodes(token);
	CREATE INDEX IF NOT EXISTS idx_nodes_owner ON nodes(owner_id);

	CREATE TABLE IF NOT EXISTS nodes_users (
		node_id TEXT NOT NULL,
		user_id INTEGER NOT NULL,
		role TEXT NOT NULL DEFAULT 'owner',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (node_id, user_id),
		FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_nodes_users_user ON nodes_users(user_id);
	`
	_, err := db.Exec(schema)
	return err
}
