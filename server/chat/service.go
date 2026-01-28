package chat

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"unblink/server/models"
	chatv1 "unblink/server/gen/chat/v1"
	"unblink/server/gen/chat/v1/chatv1connect"
)

// generateID creates a unique ID using crypto/rand
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// sanitizeForPostgres removes characters that PostgreSQL JSONB cannot handle
func sanitizeForPostgres(s string) string {
	// Remove NULL bytes and other problematic control characters
	cleaned := strings.Map(func(r rune) rune {
		// Remove NULL bytes
		if r == 0 {
			return -1
		}
		// Remove other control characters except newline, tab, carriage return
		if r < 32 && r != '\n' && r != '\t' && r != '\r' {
			return -1
		}
		// Remove unpaired surrogates (U+D800 to U+DFFF)
		if r >= 0xD800 && r <= 0xDFFF {
			return -1
		}
		return r
	}, s)

	return cleaned
}

type Service struct {
	db             Database
	openai         *openai.Client
	fastOpenai     *openai.Client
	cfg            *Config
	tools          *ToolRegistry
	modelRegistry  *models.Registry
	contentTrimmer *models.Trimmer
}

// Config holds the chat service configuration
type Config struct {
	// Main chat model (for responses)
	ChatOpenAIModel  string
	ChatOpenAIBaseURL string
	ChatOpenAIAPIKey  string

	// Fast model (for follow-up suggestions, etc.)
	FastOpenAIModel  string
	FastOpenAIBaseURL string
	FastOpenAIAPIKey  string

	// Content trimming
	ContentTrimSafetyMargin int // Percentage (0-100)
}

// Database defines the interface for chat database operations
type Database interface {
	CreateConversation(id, userID, title, systemPrompt string) error
	GetConversation(id, userID string) (*chatv1.Conversation, error)
	ListConversations(userID string) ([]*chatv1.Conversation, error)
	UpdateConversation(id, userID, title, systemPrompt string) error
	DeleteConversation(id, userID string) error
	StoreMessage(id, conversationID, body string) error
	ListMessages(conversationID, userID string) ([]*chatv1.Message, error)
	StoreUIBlock(id, conversationID, role, data string) error
	ListUIBlocks(conversationID, userID string) ([]*chatv1.UIBlock, error)
	GetSystemPrompt(conversationID, userID string) (string, error)
	GetMessagesBody(conversationID, userID string) ([]string, error)
}

func NewService(db Database, cfg *Config, modelRegistry *models.Registry) *Service {
	if cfg.ChatOpenAIModel == "" {
		cfg.ChatOpenAIModel = "gpt-4o-mini"
	}

	service := &Service{
		db:            db,
		cfg:           cfg,
		tools:         NewToolRegistry(),
		modelRegistry: modelRegistry,
	}

	if cfg.ChatOpenAIAPIKey == "" {
		log.Printf("[ChatService] WARNING: CHAT_OPENAI_API_KEY not set, using echo backend")
		return service
	}

	// Create main chat client from registry config
	mainCfg, err := modelRegistry.GetConfig(cfg.ChatOpenAIModel)
	if err != nil {
		log.Printf("[ChatService] Main model config not found: %v", err)
		return service
	}
	opts := []option.RequestOption{option.WithAPIKey(mainCfg.APIKey)}
	if mainCfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(mainCfg.BaseURL))
		log.Printf("[ChatService] Using custom base URL: %s", mainCfg.BaseURL)
	}
	client := openai.NewClient(opts...)
	service.openai = &client

	// Create fast client from registry config (if available)
	if fastCfg, err := modelRegistry.GetConfig(cfg.FastOpenAIModel); err == nil {
		fastOpts := []option.RequestOption{option.WithAPIKey(fastCfg.APIKey)}
		if fastCfg.BaseURL != "" {
			fastOpts = append(fastOpts, option.WithBaseURL(fastCfg.BaseURL))
		}
		fastClient := openai.NewClient(fastOpts...)
		service.fastOpenai = &fastClient
		log.Printf("[ChatService] Fast OpenAI client initialized with model: %s", cfg.FastOpenAIModel)
	}

	// Create trimmer for main chat model
	maxTokens := modelRegistry.GetMaxTokensOr(cfg.ChatOpenAIModel, 32000)
	service.contentTrimmer = models.NewTrimmer(maxTokens, cfg.ContentTrimSafetyMargin)
	log.Printf("[ChatService] Main model %s: max_tokens=%d, margin=%d%%",
		cfg.ChatOpenAIModel, maxTokens, cfg.ContentTrimSafetyMargin)

	return service
}

// RegisterTool registers a tool with the chat service
func (s *Service) RegisterTool(tool Tool) {
	s.tools.Register(tool)
}

// Ensure Service implements interface
var _ chatv1connect.ChatServiceHandler = (*Service)(nil)
