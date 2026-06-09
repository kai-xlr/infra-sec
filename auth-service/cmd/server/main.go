package main

import (
	"log"
	"net/http"

	"auth-service/internal/api"
	"auth-service/internal/middleware"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", api.HealthHandler)

	protected := http.NewServeMux()
	protected.HandleFunc("/whoami", api.WhoamiHandler)

	mux.Handle("/whoami", middleware.AuthMiddleware(protected))

	log.Println("Server running on :8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
