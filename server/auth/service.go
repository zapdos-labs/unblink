package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"unblink/database"
	authv1 "unblink/server/gen/chat/v1/auth"
	"unblink/server/internal/ctxutil"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TokenGenerator interface for JWT token generation
type TokenGenerator interface {
	GenerateToken(userID, email string, isGuest bool) (string, error)
}

// Service implements the AuthService
type Service struct {
	db         DB
	jwtManager TokenGenerator
}

// DB interface for user operations - matches database.Client methods
// We use the actual database package types to avoid duplication
type DB interface {
	CreateUser(id string, profile map[string]string) error
	GetUser(id string) (*database.User, error)
	IsGuest(userID string) (bool, error)
	GetAccountByUserID(userID string) (*database.Account, error)
	UpdateUserProfile(userID string, profile map[string]string) error
}

// NewService creates a new auth service
func NewService(db DB, jwtManager TokenGenerator) *Service {
	return &Service{
		db:         db,
		jwtManager: jwtManager,
	}
}

// generateID creates a unique ID using crypto/rand
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// CreateGuestUser creates a new guest user
func (s *Service) CreateGuestUser(ctx context.Context, req *connect.Request[authv1.CreateGuestUserRequest]) (*connect.Response[authv1.CreateGuestUserResponse], error) {
	// Generate user ID
	userID := generateID()

	// Create profile with optional name
	profile := make(map[string]string)
	if req.Msg.Name != nil && *req.Msg.Name != "" {
		profile["name"] = *req.Msg.Name
	}

	// Create user in database
	if err := s.db.CreateUser(userID, profile); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create user: %w", err))
	}

	// Get the created user
	user, err := s.db.GetUser(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(userID, "", true) // isGuest = true
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate token: %w", err))
	}

	// Build response
	protoUser := &authv1.User{
		Id:        user.ID,
		Profile:   user.Profile,
		IsGuest:   true,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}

	return connect.NewResponse(&authv1.CreateGuestUserResponse{
		Success: true,
		Token:    token,
		User:     protoUser,
	}), nil
}

// GetUser returns the current authenticated user
func (s *Service) GetUser(ctx context.Context, req *connect.Request[authv1.GetUserRequest]) (*connect.Response[authv1.GetUserResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Get user from database
	user, err := s.db.GetUser(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Check if user is a guest
	isGuest, err := s.db.IsGuest(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check guest status: %w", err))
	}

	// Get email from account if not guest
	var email string
	if !isGuest {
		account, err := s.db.GetAccountByUserID(userID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get account: %w", err))
		}
		if account != nil && account.Payload != nil {
			if e, ok := account.Payload["email"].(string); ok {
				email = e
			}
		}
	}

	// Build response
	protoUser := &authv1.User{
		Id:        user.ID,
		Profile:   user.Profile,
		IsGuest:   isGuest,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}
	if email != "" {
		protoUser.Email = &email
	}

	return connect.NewResponse(&authv1.GetUserResponse{
		Success: true,
		User:     protoUser,
	}), nil
}

// CreateAndLinkAccount creates a new account or links an account to an existing guest user
// Phase 2 - not implemented yet
func (s *Service) CreateAndLinkAccount(ctx context.Context, req *connect.Request[authv1.CreateAndLinkAccountRequest]) (*connect.Response[authv1.CreateAndLinkAccountResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("CreateAndLinkAccount is not implemented yet"))
}

// Login authenticates a user with email and password
// Phase 2 - not implemented yet
func (s *Service) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("Login is not implemented yet"))
}

// UpdateUserProfile updates the current user's profile
func (s *Service) UpdateUserProfile(ctx context.Context, req *connect.Request[authv1.UpdateUserProfileRequest]) (*connect.Response[authv1.UpdateUserProfileResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	// Update user profile in database
	if err := s.db.UpdateUserProfile(userID, req.Msg.Profile); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update profile: %w", err))
	}

	// Get updated user from database
	user, err := s.db.GetUser(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Check if user is a guest
	isGuest, err := s.db.IsGuest(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check guest status: %w", err))
	}

	// Get email from account if not guest
	var email string
	if !isGuest {
		account, err := s.db.GetAccountByUserID(userID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get account: %w", err))
		}
		if account != nil && account.Payload != nil {
			if e, ok := account.Payload["email"].(string); ok {
				email = e
			}
		}
	}

	// Build response
	protoUser := &authv1.User{
		Id:        user.ID,
		Profile:   user.Profile,
		IsGuest:   isGuest,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}
	if email != "" {
		protoUser.Email = &email
	}

	return connect.NewResponse(&authv1.UpdateUserProfileResponse{
		Success: true,
		User:    protoUser,
	}), nil
}

// GetUserIDFromContext extracts the user ID from the context
// This is a convenience wrapper around ctxutil.GetUserIDFromContext for backwards compatibility
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	return ctxutil.GetUserIDFromContext(ctx)
}
