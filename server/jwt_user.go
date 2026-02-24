package server

import (
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserJWTClaims represents the JWT claims for user authentication
type UserJWTClaims struct {
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	IsGuest bool   `json:"is_guest"`
	jwt.RegisteredClaims
}

// UserJWTManager handles JWT token generation and validation for users
type UserJWTManager struct {
	secretKey string
}

// NewJWTManager creates a new UserJWTManager with the given secret key
func NewJWTManager(secretKey string) *UserJWTManager {
	return &UserJWTManager{
		secretKey: secretKey,
	}
}

// GenerateToken generates a JWT token for a user
// Guest tokens expire in 30 days, registered users in 24 hours
func (m *UserJWTManager) GenerateToken(userID, email string, isGuest bool) (string, error) {
	var expiresAt time.Time

	if isGuest {
		// Guest tokens: 30 days (to encourage account linking)
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	} else {
		// Registered users: 24 hours
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	claims := &UserJWTClaims{
		UserID:  userID,
		Email:   email,
		IsGuest: isGuest,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	log.Printf("[jwt] Generating token for user_id=%s is_guest=%v expires_at=%v", userID, isGuest, expiresAt)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.secretKey))
	if err != nil {
		log.Printf("[jwt] Failed to sign token: %v", err)
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	previewLen := 30
	if len(tokenString) < previewLen {
		previewLen = len(tokenString)
	}
	log.Printf("[jwt] Token generated: %q...", tokenString[:previewLen])
	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (m *UserJWTManager) ValidateToken(tokenString string) (*UserJWTClaims, error) {
	previewLen := 30
	if len(tokenString) < previewLen {
		previewLen = len(tokenString)
	}
	log.Printf("[jwt] Validating token: %q...", tokenString[:previewLen])

	token, err := jwt.ParseWithClaims(tokenString, &UserJWTClaims{}, func(token *jwt.Token) (any, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("[jwt] Unexpected signing method: %v", token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.secretKey), nil
	})

	if err != nil {
		log.Printf("[jwt] Failed to parse token: %v", err)
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*UserJWTClaims)
	if !ok || !token.Valid {
		log.Printf("[jwt] Invalid token claims")
		return nil, fmt.Errorf("invalid token")
	}

	log.Printf("[jwt] Token valid for user_id=%s", claims.UserID)
	return claims, nil
}

// ExtractTokenFromHeader extracts the Bearer token from the Authorization header
func ExtractTokenFromHeader(authHeader string) string {
	// Handle "Bearer <token>" format
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return authHeader
}
