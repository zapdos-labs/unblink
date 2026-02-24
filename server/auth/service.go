package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
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
	db         *database.Client
	jwtManager TokenGenerator
}

// NewService creates a new auth service
func NewService(db *database.Client, jwtManager TokenGenerator) *Service {
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
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
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
func (s *Service) CreateAndLinkAccount(ctx context.Context, req *connect.Request[authv1.CreateAndLinkAccountRequest]) (*connect.Response[authv1.CreateAndLinkAccountResponse], error) {
	email := req.Msg.Email
	password := req.Msg.Password
	name := req.Msg.Name

	// Validate input
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}
	if password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password is required"))
	}
	if len(password) < 8 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password must be at least 8 characters"))
	}

	// Check if account already exists
	existingAccount, err := s.db.GetAccountByEmail(email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check existing account: %w", err))
	}
	if existingAccount != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("account with this email already exists"))
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to hash password: %w", err))
	}

	// Get authenticated user ID (if guest user is creating account)
	var userID string
	if authUserID, ok := ctxutil.GetUserIDFromContext(ctx); ok {
		// Guest user linking account
		userID = authUserID

		// Update profile with name if provided
		if name != nil && *name != "" {
			if err := s.db.UpdateUserProfile(userID, map[string]string{"name": *name}); err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update user profile: %w", err))
			}
		}
	} else {
		// New user registration
		userID = generateID()

		// Create profile with name if provided
		profile := make(map[string]string)
		if name != nil && *name != "" {
			profile["name"] = *name
		}

		// Create user in database
		if err := s.db.CreateUser(userID, profile); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create user: %w", err))
		}
	}

	// Create email/password account
	accountID := generateID()
	payload := map[string]any{
		"email":         email,
		"password_hash": string(hashedPassword),
	}

	if err := s.db.CreateAccount(accountID, userID, "email_password", payload); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create account: %w", err))
	}

	// Get the user
	user, err := s.db.GetUser(userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Generate JWT token (non-guest now)
	token, err := s.jwtManager.GenerateToken(userID, email, false) // isGuest = false
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate token: %w", err))
	}

	// Build response
	protoUser := &authv1.User{
		Id:      user.ID,
		Profile: user.Profile,
		IsGuest: false,
		Email:   &email,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}

	return connect.NewResponse(&authv1.CreateAndLinkAccountResponse{
		Success: true,
		Token:    token,
		User:     protoUser,
	}), nil
}

// Login authenticates a user with email and password
func (s *Service) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	email := req.Msg.Email
	password := req.Msg.Password

	// Validate input
	if email == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("email is required"))
	}
	if password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("password is required"))
	}

	// Get account by email
	account, err := s.db.GetAccountByEmail(email)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get account: %w", err))
	}
	if account == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid email or password"))
	}

	// Verify password
	storedHash, ok := account.Payload["password_hash"].(string)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid account data"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid email or password"))
	}

	// Get user
	user, err := s.db.GetUser(account.UserID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get user: %w", err))
	}
	if user == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("user not found"))
	}

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(user.ID, email, false) // isGuest = false
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to generate token: %w", err))
	}

	// Build response
	protoUser := &authv1.User{
		Id:        user.ID,
		Profile:   user.Profile,
		IsGuest:   false,
		Email:     &email,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}

	return connect.NewResponse(&authv1.LoginResponse{
		Success: true,
		Token:    token,
		User:     protoUser,
	}), nil
}

// UpdateUserProfile updates the current user's profile
func (s *Service) UpdateUserProfile(ctx context.Context, req *connect.Request[authv1.UpdateUserProfileRequest]) (*connect.Response[authv1.UpdateUserProfileResponse], error) {
	// Get user ID from context (set by auth interceptor)
	userID, ok := ctxutil.GetUserIDFromContext(ctx)
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

