package database

import (
	"fmt"
	"strings"
)

// CreateSchema creates the necessary tables
func (c *Client) CreateSchema() error {
	// Auth tables (users, accounts) - must be first since conversations reference users
	if _, err := c.db.Exec(createAuthTablesSQL); err != nil {
		return fmt.Errorf("failed to create auth tables: %w", err)
	}
	if _, err := c.db.Exec(createChatTablesSQL); err != nil {
		return fmt.Errorf("failed to create chat tables: %w", err)
	}
	if _, err := c.db.Exec(createServiceTablesSQL); err != nil {
		return fmt.Errorf("failed to create service tables: %w", err)
	}
	if _, err := c.db.Exec(createUserNodeTablesSQL); err != nil {
		return fmt.Errorf("failed to create user_node tables: %w", err)
	}
	if _, err := c.db.Exec(createStorageTablesSQL); err != nil {
		return fmt.Errorf("failed to create storage tables: %w", err)
	}
	return nil
}

// DropSchema drops all tables
func (c *Client) DropSchema() error {
	if _, err := c.db.Exec(dropUserNodeTablesSQL); err != nil {
		return fmt.Errorf("failed to drop user_node tables: %w", err)
	}
	if _, err := c.db.Exec(dropStorageTablesSQL); err != nil {
		return fmt.Errorf("failed to drop storage tables: %w", err)
	}
	if _, err := c.db.Exec(dropServiceTablesSQL); err != nil {
		return fmt.Errorf("failed to drop service tables: %w", err)
	}
	if _, err := c.db.Exec(dropChatTablesSQL); err != nil {
		return fmt.Errorf("failed to drop chat tables: %w", err)
	}
	if _, err := c.db.Exec(dropAuthTablesSQL); err != nil {
		return fmt.Errorf("failed to drop auth tables: %w", err)
	}
	return nil
}

// isDuplicateError checks if an error is a duplicate/unique constraint error
func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "23505")
}
