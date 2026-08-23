// Package jwt provides JSON Web Token (JWT) generation and validation functionality.
// It uses HS256 signing method and supports custom claims with user identification.
package jwt

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT handles JWT token generation and parsing operations.
type JWT struct {
	config Config
}

// NewJWT creates a JWT signer from cfg, returning an error when the
// configuration is unusable.
func NewJWT(cfg Config) (*JWT, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("jwt config: %w", err)
	}

	return &JWT{config: cfg}, nil
}

// MustNewJWT is NewJWT for callers that cannot handle a failure, such as a
// package-level variable. It panics on an invalid configuration, so prefer
// NewJWT anywhere an error can be returned.
func MustNewJWT(cfg Config) *JWT {
	j, err := NewJWT(cfg)
	if err != nil {
		panic(err)
	}

	return j
}

// GenerateToken creates a new JWT token for the specified user ID with an expiration time.
// The token is signed using HS256 algorithm with the configured secret.
// Returns the signed token string or an error if signing fails.
func (j *JWT) GenerateToken(userID uint, expiresAt time.Time) (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, CustomClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Audience:  []string{},
		},
	})

	// Sign and get the complete encoded token as a string using the key
	tokenString, err := token.SignedString([]byte(j.config.Secret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// ParseToken validates and parses a JWT token string.
// It automatically strips "Bearer " prefix if present and validates the token signature,
// expiration, and signing method. Returns the custom claims if valid, or an error otherwise.
func (j *JWT) ParseToken(tokenString string) (*CustomClaims, error) {
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, errors.New("token is empty")
	}

	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", token.Header["alg"])
		}

		return []byte(j.config.Secret), nil
	},
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		// The previous code returned the (nil) parse error here, which surfaced
		// as a nil claims pointer and a nil error.
		return nil, errors.New("token claims are not valid")
	}

	return claims, nil
}
