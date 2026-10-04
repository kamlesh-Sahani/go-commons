package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents standard authentication claims across services.
type JWTClaims struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	TenantID string `json:"tenantId,omitempty"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a signed HS256 JWT token with standard expiration.
func GenerateJWT(claims JWTClaims, secretKey string, duration time.Duration) (string, error) {
	if secretKey == "" {
		return "", errors.New("jwt secret key cannot be empty")
	}

	now := time.Now().UTC()
	claims.IssuedAt = jwt.NewNumericDate(now)
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(duration))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign jwt token: %w", err)
	}

	return tokenString, nil
}

// ValidateJWT parses and cryptographically validates a JWT token string.
func ValidateJWT(tokenStr, secretKey string) (*JWTClaims, error) {
	if secretKey == "" {
		return nil, errors.New("jwt secret key cannot be empty")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid or expired token claims")
	}

	return claims, nil
}
