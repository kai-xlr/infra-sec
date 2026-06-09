package auth

import (
	"errors"
	"time"

	"auth-service/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("your-highly-secure-secret-key-change-in-production")

func ValidateToken(tokenStr string) (*models.CustomClaims, error) {
	claims := &models.CustomClaims{}

	token, err := jwt.ParseWithClaims(
		tokenStr,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return jwtSecret, nil
		},
	)
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	if claims.Issuer != "auth-service" {
		return nil, errors.New("invalid issuer")
	}

	if claims.ExpiresAt == nil {
		return nil, errors.New("missing expiration")
	}

	if claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("token expired")
	}

	return claims, nil
}
