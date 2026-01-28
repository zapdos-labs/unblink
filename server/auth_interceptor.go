package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"unblink/server/internal/ctxutil"

	connect "connectrpc.com/connect"
)

// AuthInterceptor validates JWT tokens and adds user info to context
type AuthInterceptor struct {
	jwtManager *UserJWTManager
}

// NewAuthInterceptor creates a new auth interceptor
func NewAuthInterceptor(jwtManager *UserJWTManager) *AuthInterceptor {
	return &AuthInterceptor{
		jwtManager: jwtManager,
	}
}

// WrapUnary implements the connect.UnaryInterceptorFunc interface
func (i *AuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		// Skip auth for CreateGuestUser
		if req.Spec().Procedure == "/chat.v1.auth.AuthService/CreateGuestUser" {
			return next(ctx, req)
		}

		// Extract and validate token
		newCtx, err := i.authenticate(ctx, req.Header())
		if err != nil {
			return nil, err
		}

		return next(newCtx, req)
	})
}

// WrapStreamingHandler implements the connect.StreamingHandlerInterceptor interface
func (i *AuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return connect.StreamingHandlerFunc(func(ctx context.Context, shc connect.StreamingHandlerConn) error {
		// Skip auth for CreateGuestUser
		if shc.Spec().Procedure == "/chat.v1.auth.AuthService/CreateGuestUser" {
			return next(ctx, shc)
		}

		// Extract and validate token
		newCtx, err := i.authenticate(ctx, shc.RequestHeader())
		if err != nil {
			return err
		}

		return next(newCtx, shc)
	})
}

// WrapStreamingClient implements the connect.StreamingClientInterceptor interface
func (i *AuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // Client-side streaming doesn't need auth for now
}

// authenticate validates the JWT token and adds user info to context
func (i *AuthInterceptor) authenticate(ctx context.Context, header http.Header) (context.Context, error) {
	// Get Authorization header
	authHeader := header.Get("Authorization")
	log.Printf("[auth] Authorization header: %q", authHeader)
	if authHeader == "" {
		log.Printf("[auth] Missing authorization header")
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("missing authorization header"))
	}

	// Extract Bearer token
	tokenString := ExtractTokenFromHeader(authHeader)
	log.Printf("[auth] Extracted token: %q...", tokenString[:min(30, len(tokenString))])
	if tokenString == "" {
		log.Printf("[auth] Invalid authorization header format")
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid authorization header format"))
	}

	// Validate token
	claims, err := i.jwtManager.ValidateToken(tokenString)
	if err != nil {
		log.Printf("[auth] Token validation failed: %v", err)
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("invalid token: %w", err))
	}
	log.Printf("[auth] Token validated for user_id: %s", claims.UserID)

	// Check if token is expired
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		log.Printf("[auth] Token expired for user_id: %s", claims.UserID)
		return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("token expired"))
	}

	// Add user ID and claims to context
	newCtx := context.WithValue(ctx, ctxutil.ContextKeyUserID, claims.UserID)
	newCtx = context.WithValue(newCtx, ctxutil.ContextKeyClaims, claims)

	log.Printf("[auth] Authentication successful for user_id: %s", claims.UserID)
	return newCtx, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetClaimsFromContext extracts the JWT claims from the context
func GetClaimsFromContext(ctx context.Context) (*UserJWTClaims, bool) {
	claims, ok := ctx.Value(ctxutil.ContextKeyClaims).(*UserJWTClaims)
	return claims, ok
}
