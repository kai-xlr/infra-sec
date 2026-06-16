package main

import (
	"log"
	"net/http"

	"auth-service/internal/api"
	"auth-service/internal/audit"
	"auth-service/internal/middleware"
)

func main() {
	auditLogger, err := audit.NewLogger("audit.log")
	if err != nil {
		log.Fatal(err)
	}
	defer auditLogger.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", api.HealthHandler)
	mux.HandleFunc("/auth/login", api.LoginHandler)

	protected := http.NewServeMux()
	protected.HandleFunc("/whoami", api.WhoamiHandler)

	projects := http.HandlerFunc(api.ProjectsHandler)

	readProjects :=
		middleware.RequirePermission("read", "project", auditLogger)(projects)

	writeProjects :=
		middleware.RequirePermission("write", "project", auditLogger)(projects)

	deleteProjects :=
		middleware.RequirePermission("delete", "project", auditLogger)(projects)

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
