package relay

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"time"
)

// Node represents a node in the system
type Node struct {
	ID              string
	Token           string
	OwnerID         *int64
	Name            *string
	CreatedAt       time.Time
	AuthorizedAt    *time.Time
	LastConnectedAt *time.Time
}

// NodeTable handles node data access
type NodeTable struct {
	db *sql.DB
}

// NewNodeTable creates a new NodeTable
func NewNodeTable(db *sql.DB) *NodeTable {
	return &NodeTable{db: db}
}

// generateSecureToken creates a cryptographically secure random token
func generateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// AuthorizeNode creates a node with token and links it to a user
func (s *NodeTable) AuthorizeNode(nodeID string, ownerID int64, name, token string) error {
	// Insert node with owner and token
	result, err := s.db.Exec(
		"INSERT OR REPLACE INTO nodes (id, token, owner_id, name, authorized_at) VALUES (?, ?, ?, ?, ?)",
		nodeID, token, ownerID, name, time.Now(),
	)
	if err != nil {
		return err
	}

	// Also add to nodes_users junction table
	_, err = s.db.Exec(
		"INSERT OR REPLACE INTO nodes_users (node_id, user_id, role) VALUES (?, ?, 'owner')",
		nodeID, ownerID,
	)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return errors.New("node not found")
	}

	log.Printf("[NodeTable] Node %s authorized by user %d", nodeID, ownerID)
	return nil
}

// GetNodeByToken retrieves a node by its authorization token
func (s *NodeTable) GetNodeByToken(token string) (*Node, error) {
	var node Node
	var ownerID sql.NullInt64
	var name sql.NullString
	var authorizedAt, lastConnected sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, token, owner_id, name, created_at, authorized_at, last_connected_at
		FROM nodes WHERE token = ?
	`, token).Scan(
		&node.ID, &node.Token, &ownerID, &name, &node.CreatedAt,
		&authorizedAt, &lastConnected,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("invalid token")
	}
	if err != nil {
		return nil, err
	}

	if ownerID.Valid {
		node.OwnerID = &ownerID.Int64
	}
	if name.Valid {
		node.Name = &name.String
	}
	if authorizedAt.Valid {
		node.AuthorizedAt = &authorizedAt.Time
	}
	if lastConnected.Valid {
		node.LastConnectedAt = &lastConnected.Time
	}

	return &node, nil
}

// GetNodeByID retrieves a node by its ID
func (s *NodeTable) GetNodeByID(nodeID string) (*Node, error) {
	var node Node
	var ownerID sql.NullInt64
	var name sql.NullString
	var authorizedAt, lastConnected sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, token, owner_id, name, created_at, authorized_at, last_connected_at
		FROM nodes WHERE id = ?
	`, nodeID).Scan(
		&node.ID, &node.Token, &ownerID, &name, &node.CreatedAt,
		&authorizedAt, &lastConnected,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("node not found")
	}
	if err != nil {
		return nil, err
	}

	if ownerID.Valid {
		node.OwnerID = &ownerID.Int64
	}
	if name.Valid {
		node.Name = &name.String
	}
	if authorizedAt.Valid {
		node.AuthorizedAt = &authorizedAt.Time
	}
	if lastConnected.Valid {
		node.LastConnectedAt = &lastConnected.Time
	}

	return &node, nil
}

// GetNodesByUser retrieves all nodes owned by a user
func (s *NodeTable) GetNodesByUser(userID int64) ([]*Node, error) {
	rows, err := s.db.Query(`
		SELECT id, token, owner_id, name, created_at, authorized_at, last_connected_at
		FROM nodes WHERE owner_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		var node Node
		var ownerID sql.NullInt64
		var name sql.NullString
		var authorizedAt, lastConnected sql.NullTime

		err := rows.Scan(
			&node.ID, &node.Token, &ownerID, &name, &node.CreatedAt,
			&authorizedAt, &lastConnected,
		)
		if err != nil {
			return nil, err
		}

		if ownerID.Valid {
			node.OwnerID = &ownerID.Int64
		}
		if name.Valid {
			node.Name = &name.String
		}
		if authorizedAt.Valid {
			node.AuthorizedAt = &authorizedAt.Time
		}
		if lastConnected.Valid {
			node.LastConnectedAt = &lastConnected.Time
		}

		nodes = append(nodes, &node)
	}

	return nodes, nil
}

// UpdateLastConnected updates the last connected timestamp
func (s *NodeTable) UpdateLastConnected(nodeID string) error {
	_, err := s.db.Exec(
		"UPDATE nodes SET last_connected_at = ? WHERE id = ?",
		time.Now(), nodeID,
	)
	return err
}

// UpdateNodeName updates the name of a node
func (s *NodeTable) UpdateNodeName(nodeID string, name string) error {
	_, err := s.db.Exec(
		"UPDATE nodes SET name = ? WHERE id = ?",
		name, nodeID,
	)
	return err
}

// DeleteNode removes a node (unauthorizes it)
func (s *NodeTable) DeleteNode(nodeID string) error {
	_, err := s.db.Exec("DELETE FROM nodes WHERE id = ?", nodeID)
	return err
}

// UserOwnsNode checks if a user owns a specific node
func (s *NodeTable) UserOwnsNode(userID int64, nodeID string) bool {
	var ownerID sql.NullInt64
	err := s.db.QueryRow("SELECT owner_id FROM nodes WHERE id = ?", nodeID).Scan(&ownerID)
	if err != nil {
		return false
	}
	return ownerID.Valid && ownerID.Int64 == userID
}
