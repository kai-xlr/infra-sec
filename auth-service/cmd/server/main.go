package main

import (
	"log"
	"net/http"

	"auth-service/internal/api"
	"auth-service/internal/audit"
	"auth-service/internal/middleware"
	"auth-service/internal/store"
)

func main() {
	auditLogger, err := audit.NewLogger("audit.log")
	if err != nil {
		log.Fatal(err)
	}
	defer auditLogger.Close()

	memStore := store.NewInMemoryStore()

	adminHash := "$2a$10$dv0AcULv0j9unVsTZvIxpeaGYLryIi17tEiiZp./dUm4Ab8fXQvqq"
	_, err = memStore.CreateUser("admin", adminHash, "admin")
	if err != nil {
		log.Fatalf("Failed to seed admin user: %v", err)
	}

	authHandler := api.NewAuthHandler(memStore)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", api.HealthHandler)
	mux.HandleFunc("/auth/login", authHandler.LoginHandler)

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

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/admin/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			middleware.RequirePermission("read", "admin", auditLogger)(
				http.HandlerFunc(authHandler.ListUsersHandler),
			).ServeHTTP(w, r)
		case http.MethodPost:
			middleware.RequirePermission("write", "admin", auditLogger)(
				http.HandlerFunc(authHandler.CreateUserHandler),
			).ServeHTTP(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.Handle(
		"/admin/users",
		middleware.AuthMiddleware(adminMux),
	)

	mux.Handle(
		"/admin/users/{id}",
		middleware.AuthMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					middleware.RequirePermission("read", "admin", auditLogger)(
						http.HandlerFunc(authHandler.GetUserHandler),
					).ServeHTTP(w, r)
				case http.MethodPut:
					middleware.RequirePermission("write", "admin", auditLogger)(
						http.HandlerFunc(authHandler.UpdateUserHandler),
					).ServeHTTP(w, r)
				case http.MethodDelete:
					middleware.RequirePermission("delete", "admin", auditLogger)(
						http.HandlerFunc(authHandler.DeleteUserHandler),
					).ServeHTTP(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
			}),
		),
	)

	log.Println("Server running on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
