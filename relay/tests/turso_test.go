package relay_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/tursodatabase/turso-go"
)

// TestTursoBasicConnect tests basic Turso database connectivity
func TestTursoBasicConnect(t *testing.T) {
	// Create a temporary directory for the test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open a Turso database connection
	// Using turso driver with local SQLite file
	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Test connection with a simple ping
	if err := conn.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	t.Log("Successfully connected to Turso database")
}

// TestTursoCreateTable tests creating a table in Turso database
func TestTursoCreateTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create a test table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	t.Log("Successfully created users table")
}

// TestTursoInsertAndQuery tests inserting and querying data
func TestTursoInsertAndQuery(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	insertSQL := `INSERT INTO users (username, email) VALUES (?, ?)`
	users := []struct {
		username string
		email    string
	}{
		{"alice", "alice@example.com"},
		{"bob", "bob@example.com"},
		{"charlie", "charlie@example.com"},
	}

	for _, user := range users {
		result, err := conn.Exec(insertSQL, user.username, user.email)
		if err != nil {
			t.Fatalf("Failed to insert user %s: %v", user.username, err)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected != 1 {
			t.Errorf("Expected 1 row affected, got %d", rowsAffected)
		}
	}

	t.Logf("Successfully inserted %d users", len(users))

	// Query the data
	querySQL := `SELECT id, username, email FROM users ORDER BY username`
	rows, err := conn.Query(querySQL)
	if err != nil {
		t.Fatalf("Failed to query users: %v", err)
	}
	defer rows.Close()

	var retrievedUsers []struct {
		id       int
		username string
		email    string
	}

	for rows.Next() {
		var id int
		var username, email string
		if err := rows.Scan(&id, &username, &email); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		retrievedUsers = append(retrievedUsers, struct {
			id       int
			username string
			email    string
		}{id, username, email})
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("Error iterating rows: %v", err)
	}

	// Verify we got all users back
	if len(retrievedUsers) != len(users) {
		t.Fatalf("Expected %d users, got %d", len(users), len(retrievedUsers))
	}

	// Verify usernames match
	for i, user := range retrievedUsers {
		expectedUser := users[i]
		if user.username != expectedUser.username {
			t.Errorf("Expected username %s, got %s", expectedUser.username, user.username)
		}
		if user.email != expectedUser.email {
			t.Errorf("Expected email %s for user %s, got %s", expectedUser.email, expectedUser.username, user.email)
		}
	}

	t.Logf("Successfully queried and verified %d users", len(retrievedUsers))
}

// TestTursoUpdate tests updating records in the database
func TestTursoUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table and insert initial data
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	insertSQL := `INSERT INTO users (username, email) VALUES (?, ?)`
	if _, err := conn.Exec(insertSQL, "alice", "alice@old.com"); err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	// Update email
	updateSQL := `UPDATE users SET email = ? WHERE username = ?`
	result, err := conn.Exec(updateSQL, "alice@new.com", "alice")
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", rowsAffected)
	}

	// Verify update
	var email string
	err = conn.QueryRow(`SELECT email FROM users WHERE username = ?`, "alice").Scan(&email)
	if err != nil {
		t.Fatalf("Failed to query updated user: %v", err)
	}

	if email != "alice@new.com" {
		t.Errorf("Expected email alice@new.com, got %s", email)
	}

	t.Log("Successfully updated user email")
}

// TestTursoDelete tests deleting records from the database
func TestTursoDelete(t *testing.T) {
	// Use in-memory database for complete test isolation
	conn, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	insertSQL := `INSERT INTO users (username, email) VALUES (?, ?)`
	if _, err := conn.Exec(insertSQL, "alice", "alice@example.com"); err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	// Verify alice exists
	var count int
	err = conn.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, "alice").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count before delete: %v", err)
	}
	t.Logf("Before delete: %d alice users found", count)

	// Delete user
	deleteSQL := `DELETE FROM users WHERE username = ?`
	result, err := conn.Exec(deleteSQL, "alice")
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	t.Logf("Delete affected %d rows", rowsAffected)
	if rowsAffected < 1 {
		t.Errorf("Expected at least 1 row affected, got %d", rowsAffected)
	}

	// Verify deletion
	err = conn.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ?`, "alice").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 users, got %d", count)
	}

	t.Log("Successfully deleted user")
}

// TestTursoTransactions tests transaction support
func TestTursoTransactions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			balance INTEGER NOT NULL DEFAULT 0
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert initial accounts
	insertSQL := `INSERT INTO accounts (name, balance) VALUES (?, ?)`
	if _, err := conn.Exec(insertSQL, "alice", 100); err != nil {
		t.Fatalf("Failed to insert alice: %v", err)
	}
	if _, err := conn.Exec(insertSQL, "bob", 50); err != nil {
		t.Fatalf("Failed to insert bob: %v", err)
	}

	// Transfer funds in a transaction
	tx, err := conn.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Debit from alice
	_, err = tx.Exec(`UPDATE accounts SET balance = balance - 30 WHERE name = ?`, "alice")
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to debit alice: %v", err)
	}

	// Credit to bob
	_, err = tx.Exec(`UPDATE accounts SET balance = balance + 30 WHERE name = ?`, "bob")
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to credit bob: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify balances
	var aliceBalance, bobBalance int
	err = conn.QueryRow(`SELECT balance FROM accounts WHERE name = ?`, "alice").Scan(&aliceBalance)
	if err != nil {
		t.Fatalf("Failed to query alice balance: %v", err)
	}

	err = conn.QueryRow(`SELECT balance FROM accounts WHERE name = ?`, "bob").Scan(&bobBalance)
	if err != nil {
		t.Fatalf("Failed to query bob balance: %v", err)
	}

	if aliceBalance != 70 {
		t.Errorf("Expected alice balance 70, got %d", aliceBalance)
	}
	if bobBalance != 80 {
		t.Errorf("Expected bob balance 80, got %d", bobBalance)
	}

	t.Log("Successfully completed transaction")
}

// TestTursoPreparedStatements tests prepared statement usage
func TestTursoPreparedStatements(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			price INTEGER NOT NULL
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Use prepared statement for multiple inserts
	insertSQL := `INSERT INTO products (name, price) VALUES (?, ?)`
	stmt, err := conn.Prepare(insertSQL)
	if err != nil {
		t.Fatalf("Failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	products := []struct {
		name  string
		price int
	}{
		{"widget", 100},
		{"gadget", 200},
		{"doohickey", 150},
	}

	for _, product := range products {
		_, err := stmt.Exec(product.name, product.price)
		if err != nil {
			t.Fatalf("Failed to insert product %s: %v", product.name, err)
		}
	}

	t.Logf("Successfully inserted %d products using prepared statement", len(products))

	// Query using prepared statement
	querySQL := `SELECT name, price FROM products WHERE price > ? ORDER BY price`
	rows, err := conn.Query(querySQL, 120)
	if err != nil {
		t.Fatalf("Failed to query products: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var name string
		var price int
		if err := rows.Scan(&name, &price); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}
		count++
		t.Logf("Found product: %s, price: %d", name, price)
	}

	if count != 2 {
		t.Errorf("Expected 2 products with price > 120, got %d", count)
	}
}

// TestTursoWithRealDatabase tests with a real database file
// This test is skipped by default and can be run with:
// go test -run TestTursoWithRealDatabase ./relay/tests/
func TestTursoWithRealDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real database test in short mode")
	}

	// Use a persistent database file in the current directory
	dbPath := "test_relay.db"
	defer os.Remove(dbPath)

	conn, err := sql.Open("turso", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	// Create a relay nodes table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS relay_nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			address TEXT NOT NULL,
			last_heartbeat DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT DEFAULT 'active'
		)
	`

	if _, err := conn.Exec(createTableSQL); err != nil {
		t.Fatalf("Failed to create relay_nodes table: %v", err)
	}

	// Insert a test node
	insertSQL := `INSERT INTO relay_nodes (id, name, address) VALUES (?, ?, ?)`
	nodeID := fmt.Sprintf("node-%d", 12345)
	if _, err := conn.Exec(insertSQL, nodeID, "test-node", "192.168.1.100:8080"); err != nil {
		t.Fatalf("Failed to insert node: %v", err)
	}

	// Query the node
	var name, address string
	err = conn.QueryRow(`SELECT name, address FROM relay_nodes WHERE id = ?`, nodeID).Scan(&name, &address)
	if err != nil {
		t.Fatalf("Failed to query node: %v", err)
	}

	if name != "test-node" {
		t.Errorf("Expected name 'test-node', got '%s'", name)
	}
	if address != "192.168.1.100:8080" {
		t.Errorf("Expected address '192.168.1.100:8080', got '%s'", address)
	}

	t.Logf("Successfully tested with real database file: %s", dbPath)
}
