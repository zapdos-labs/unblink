package database

import (
	"database/sql"
	"fmt"
	"time"

	chatv1 "unblink/server/gen/chat/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// DefaultSystemPrompt is the default system prompt for all conversations
const DefaultSystemPrompt = "You are a helpful assistant named Unblink. You are created by Zapdos Labs. You have access to camera events and can help users analyze and summarize their video footages."

const (
	createChatTablesSQL = `
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			title TEXT NOT NULL,
			system_prompt TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			conversation_id TEXT REFERENCES conversations(id) ON DELETE CASCADE,
			body JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS ui_blocks (
			id TEXT PRIMARY KEY,
			conversation_id TEXT REFERENCES conversations(id) ON DELETE CASCADE,
			role TEXT NOT NULL,
			data JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_ui_blocks_conversation ON ui_blocks(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_conversations_updated ON conversations(updated_at DESC);
		CREATE INDEX IF NOT EXISTS idx_conversations_user_id ON conversations(user_id);
	`

	dropChatTablesSQL = `DROP TABLE IF EXISTS ui_blocks, messages, conversations CASCADE`
)

// CreateConversation creates a new conversation
func (c *Client) CreateConversation(id, userID, title string) error {
	// Always use DefaultSystemPrompt from chat package
	insertSQL := `
		INSERT INTO conversations (id, user_id, title, system_prompt)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			title = EXCLUDED.title,
			system_prompt = EXCLUDED.system_prompt,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := c.db.Exec(insertSQL, id, userID, title, DefaultSystemPrompt)
	if err != nil {
		return fmt.Errorf("failed to create conversation: %w", err)
	}

	return nil
}

// GetConversation retrieves a single conversation by ID
func (c *Client) GetConversation(id string) (*chatv1.Conversation, error) {
	querySQL := `
		SELECT id, title, created_at, updated_at
		FROM conversations
		WHERE id = $1
	`

	var conv chatv1.Conversation
	var title sql.NullString
	var createdAt, updatedAt time.Time

	err := c.db.QueryRow(querySQL, id).Scan(
		&conv.Id,
		&title,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	if title.Valid {
		conv.Title = title.String
	}

	conv.CreatedAt = timestampToProto(createdAt)
	conv.UpdatedAt = timestampToProto(updatedAt)

	return &conv, nil
}

// ListConversations retrieves all conversations for a user ordered by updated_at
func (c *Client) ListConversations(userID string) ([]*chatv1.Conversation, error) {
	querySQL := `
		SELECT id, title, created_at, updated_at
		FROM conversations
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`

	rows, err := c.db.Query(querySQL, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	var conversations []*chatv1.Conversation

	for rows.Next() {
		var conv chatv1.Conversation
		var title sql.NullString
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&conv.Id,
			&title,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}

		if title.Valid {
			conv.Title = title.String
		}

		conv.CreatedAt = timestampToProto(createdAt)
		conv.UpdatedAt = timestampToProto(updatedAt)

		conversations = append(conversations, &conv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating conversations: %w", err)
	}

	return conversations, nil
}

// UpdateConversation updates a conversation's title or system prompt
func (c *Client) UpdateConversation(id, title string) error {
	updateSQL := `
		UPDATE conversations
		SET
			title = $1,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	_, err := c.db.Exec(updateSQL, title, id)
	if err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	return nil
}

// DeleteConversation removes a conversation and all associated messages/UI blocks
func (c *Client) DeleteConversation(id string) error {
	deleteSQL := `DELETE FROM conversations WHERE id = $1`

	_, err := c.db.Exec(deleteSQL, id)
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	return nil
}

// StoreMessage saves a message to the database
func (c *Client) StoreMessage(id, conversationID, body string) error {
	insertSQL := `
		INSERT INTO messages (id, conversation_id, body)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (id) DO NOTHING
	`

	_, err := c.db.Exec(insertSQL, id, conversationID, body)
	if err != nil {
		return fmt.Errorf("failed to store message: %w", err)
	}

	return nil
}

// ListMessages retrieves all messages for a conversation
func (c *Client) ListMessages(conversationID string) ([]*chatv1.Message, error) {
	querySQL := `
		SELECT id, conversation_id, body, created_at
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC
	`

	rows, err := c.db.Query(querySQL, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	defer rows.Close()

	var messages []*chatv1.Message

	for rows.Next() {
		var msg chatv1.Message
		var body sql.NullString
		var createdAt time.Time

		if err := rows.Scan(
			&msg.Id,
			&msg.ConversationId,
			&body,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		if body.Valid {
			msg.Body = body.String
		}

		msg.CreatedAt = timestampToProto(createdAt)

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// StoreUIBlock saves a UI block to the database
func (c *Client) StoreUIBlock(id, conversationID, role, data string) error {
	insertSQL := `
		INSERT INTO ui_blocks (id, conversation_id, role, data)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (id) DO UPDATE SET
			data = EXCLUDED.data,
			created_at = CURRENT_TIMESTAMP
	`

	_, err := c.db.Exec(insertSQL, id, conversationID, role, data)
	if err != nil {
		return fmt.Errorf("failed to store UI block: %w", err)
	}

	return nil
}

// ListUIBlocks retrieves all UI blocks for a conversation
func (c *Client) ListUIBlocks(conversationID string) ([]*chatv1.UIBlock, error) {
	querySQL := `
		SELECT id, conversation_id, role, data, created_at
		FROM ui_blocks
		WHERE conversation_id = $1
		ORDER BY created_at ASC
	`

	rows, err := c.db.Query(querySQL, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to list UI blocks: %w", err)
	}
	defer rows.Close()

	var blocks []*chatv1.UIBlock

	for rows.Next() {
		var block chatv1.UIBlock
		var data sql.NullString
		var createdAt time.Time

		if err := rows.Scan(
			&block.Id,
			&block.ConversationId,
			&block.Role,
			&data,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan UI block: %w", err)
		}

		if data.Valid {
			block.Data = data.String
		}

		block.CreatedAt = timestampToProto(createdAt)

		blocks = append(blocks, &block)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating UI blocks: %w", err)
	}

	return blocks, nil
}

// timestampToProto converts a time.Time to protobuf Timestamp
func timestampToProto(t time.Time) *timestamppb.Timestamp {
	return &timestamppb.Timestamp{Seconds: t.Unix(), Nanos: int32(t.Nanosecond())}
}

// GetSystemPrompt retrieves the system prompt for a conversation
func (c *Client) GetSystemPrompt(conversationID string) (string, error) {
	var systemPrompt sql.NullString
	err := c.db.QueryRow("SELECT system_prompt FROM conversations WHERE id = $1", conversationID).Scan(&systemPrompt)
	if err != nil {
		return "", err
	}
	if systemPrompt.Valid {
		return systemPrompt.String, nil
	}
	return "", nil
}

// GetMessagesBody retrieves all message bodies for a conversation
func (c *Client) GetMessagesBody(conversationID string) ([]string, error) {
	querySQL := `
		SELECT body
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC
	`

	rows, err := c.db.Query(querySQL, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message bodies: %w", err)
	}
	defer rows.Close()

	var bodies []string
	for rows.Next() {
		var body sql.NullString
		if err := rows.Scan(&body); err != nil {
			return nil, fmt.Errorf("failed to scan message body: %w", err)
		}
		if body.Valid {
			bodies = append(bodies, body.String)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating message bodies: %w", err)
	}

	return bodies, nil
}

// GetConversationOwner retrieves the user_id for a conversation (for authorization)
func (c *Client) GetConversationOwner(conversationID string) (string, error) {
	var userID string
	err := c.db.QueryRow("SELECT user_id FROM conversations WHERE id = $1", conversationID).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Conversation doesn't exist
		}
		return "", fmt.Errorf("failed to get conversation owner: %w", err)
	}
	return userID, nil
}
