package ctxutil

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
)

// Context key type for user ID
type contextKey string

const (
	UserIDKey        contextKey = "userID"
	ContextKeyUserID contextKey = "userID"
	ContextKeyClaims contextKey = "claims"
)

// GetUserIDFromContext extracts the user ID from the context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	if !ok || userID == "" {
		return "", false
	}
	return userID, true
}

// SetUserIDInContext sets the user ID in the context
func SetUserIDInContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// MustGetUserIDFromContext extracts the user ID or panics
func MustGetUserIDFromContext(ctx context.Context) string {
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		panic("user ID not found in context")
	}
	return userID
}

// GetRequiredUserIDFromContext extracts the user ID and returns an error if not found
func GetRequiredUserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("user ID not found in context")
	}
	return userID, nil
}

// NodeAccessChecker defines the interface for checking node access
type NodeAccessChecker interface {
	CheckNodeAccess(nodeID, userID string) (bool, error)
}

// CheckNodeAccessWithContext checks if the user can access the node.
// A node is accessible if it's public (no users associated) or the user is associated with it.
func CheckNodeAccessWithContext(ctx context.Context, db NodeAccessChecker, nodeID string) error {
	userID, ok := GetUserIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("not authenticated"))
	}

	hasAccess, err := db.CheckNodeAccess(nodeID, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to verify node access: %w", err))
	}

	if !hasAccess {
		return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you don't have access to this node"))
	}

	return nil
}
