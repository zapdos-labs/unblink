package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// NodeJWTClaims represents JWT claims structure for node tokens
type NodeJWTClaims struct {
	NodeID string `json:"node_id"`
	jwt.RegisteredClaims
}

// GenerateNodeToken creates a new JWT token for a node
func GenerateNodeToken(nodeID, secret string) (string, error) {
	if secret == "" {
		return "", errors.New("JWT secret not configured")
	}

	// Create claims with no expiration (token is valid indefinitely)
	claims := NodeJWTClaims{
		NodeID: nodeID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateNodeToken validates a JWT token and returns the node claims
func ValidateNodeToken(tokenString, secret string) (*NodeJWTClaims, error) {
	if secret == "" {
		return nil, errors.New("JWT secret not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &NodeJWTClaims{}, func(token *jwt.Token) (any, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*NodeJWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("token expired")
	}

	return claims, nil
}
