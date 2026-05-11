package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Manager struct {
	secret []byte
}

type Claims struct {
	UserID string `json:"userId"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) IssueToken(userID, tokenType string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID: userID,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	})
	signed, err := token.SignedString(m.secret)
	return signed, expiresAt, err
}

func (m *Manager) Parse(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
