package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"auth-service/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("your-highly-secure-secret-key-change-in-production")

func main() {
	role := flag.String("role", "admin", "role to encode (admin, developer, viewer)")
	sub := flag.String("sub", "test-user", "subject claim")
	dur := flag.Duration("dur", 1*time.Hour, "token duration")

	flag.Parse()

	now := time.Now()
	claims := models.CustomClaims{
		Role: *role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   *sub,
			Issuer:    "auth-service",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(*dur)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(signed)
}
