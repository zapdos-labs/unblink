package database

import (
	"database/sql"
	"fmt"
)

// AssociateUserNode adds a user-node association (makes node private to that user)
func (c *Client) AssociateUserNode(userID, nodeID string) error {
	insertSQL := `
		INSERT INTO user_node (user_id, node_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, node_id) DO NOTHING
	`

	_, err := c.db.Exec(insertSQL, userID, nodeID)
	if err != nil {
		return fmt.Errorf("failed to associate user with node: %w", err)
	}

	return nil
}

// DissociateUserNode removes user-node association
func (c *Client) DissociateUserNode(userID, nodeID string) error {
	deleteSQL := `DELETE FROM user_node WHERE user_id = $1 AND node_id = $2`

	result, err := c.db.Exec(deleteSQL, userID, nodeID)
	if err != nil {
		return fmt.Errorf("failed to dissociate user from node: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// CheckNodeAccess checks if a user can access a node.
// A node is accessible if:
// 1. It's public (no users associated), OR
// 2. The user is explicitly associated with the node
func (c *Client) CheckNodeAccess(nodeID, userID string) (bool, error) {
	// Check if node has any users associated (private node)
	var hasUsers bool
	checkSQL := `SELECT EXISTS(SELECT 1 FROM user_node WHERE node_id = $1)`
	err := c.db.QueryRow(checkSQL, nodeID).Scan(&hasUsers)
	if err != nil {
		return false, fmt.Errorf("failed to check node status: %w", err)
	}

	// If node is public (no users associated), allow access
	if !hasUsers {
		return true, nil
	}

	// Node is private - check if user is associated
	var isAssociated bool
	assocSQL := `SELECT EXISTS(SELECT 1 FROM user_node WHERE node_id = $1 AND user_id = $2)`
	err = c.db.QueryRow(assocSQL, nodeID, userID).Scan(&isAssociated)
	if err != nil {
		return false, fmt.Errorf("failed to check node association: %w", err)
	}

	return isAssociated, nil
}

// ListUserNodes returns all node IDs associated with a user
func (c *Client) ListUserNodes(userID string) ([]string, error) {
	querySQL := `SELECT node_id FROM user_node WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := c.db.Query(querySQL, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user nodes: %w", err)
	}
	defer rows.Close()

	var nodeIDs []string
	for rows.Next() {
		var nodeID string
		if err := rows.Scan(&nodeID); err != nil {
			return nil, fmt.Errorf("failed to scan node_id: %w", err)
		}
		nodeIDs = append(nodeIDs, nodeID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return nodeIDs, nil
}
