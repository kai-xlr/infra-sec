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

	projects := http.HandlerFunc(api.ProjectsHandler)

	readProjects :=
		middleware.RequirePermission("read")(projects)

	writeProjects :=
		middleware.RequirePermission("write")(projects)

	deleteProjects :=
		middleware.RequirePermission("delete")(projects)

	protected.Handle("/projects", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {

		case http.MethodGet:
			readProjects.ServeHTTP(w, r)

		case http.MethodPost:
			writeProjects.ServeHTTP(w, r)

		case http.MethodDelete:
			deleteProjects.ServeHTTP(w, r)

		default:
			projects.ServeHTTP(w, r)
		}
	}))

	mux.Handle(
		"/whoami",
		middleware.AuthMiddleware(protected),
	)

	mux.Handle(
		"/projects",
		middleware.AuthMiddleware(protected),
	)

	log.Println("Server running on :8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
