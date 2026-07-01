package model

import (
	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	Role     string `json:"role"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type ContextKey string

const ClaimsKey ContextKey = "claims"
