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
	adminMux.Handle("/admin/users", middleware.RequireRole("admin", auditLogger)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				authHandler.ListUsersHandler(w, r)
			case http.MethodPost:
				authHandler.CreateUserHandler(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}),
	))

	mux.Handle(
		"/admin/users",
		middleware.AuthMiddleware(adminMux),
	)

	mux.Handle(
		"/admin/users/{id}",
		middleware.AuthMiddleware(
			middleware.RequireRole("admin", auditLogger)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.Method {
					case http.MethodGet:
						authHandler.GetUserHandler(w, r)
					case http.MethodPut:
						authHandler.UpdateUserHandler(w, r)
					case http.MethodDelete:
						authHandler.DeleteUserHandler(w, r)
					default:
						http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					}
				}),
			),
		),
	)

	log.Println("Server running on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
