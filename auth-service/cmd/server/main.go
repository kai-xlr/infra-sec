package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"auth-service/internal/handler"
	"auth-service/internal/audit"
	"auth-service/internal/middleware"
	"auth-service/internal/store"
	"auth-service/internal/worker"
)

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func main() {
	auditLogger, err := audit.NewLogger("audit.log")
	if err != nil {
		log.Fatal(err)
	}
	defer auditLogger.Close()

	sqlStore, err := store.NewSQLiteStore("users.db")
	if err != nil {
		log.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer sqlStore.Close()

	adminHash := "$2a$10$dv0AcULv0j9unVsTZvIxpeaGYLryIi17tEiiZp./dUm4Ab8fXQvqq"
	_, err = sqlStore.CreateUser("admin", adminHash, "admin")
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			log.Fatalf("Failed to seed admin user: %v", err)
		}
	}

	authHandler := handler.NewAuthHandler(sqlStore)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", handler.HealthHandler)
	mux.HandleFunc("/auth/login", authHandler.LoginHandler)

	protected := http.NewServeMux()
	protected.HandleFunc("/whoami", handler.WhoamiHandler)

	projects := http.HandlerFunc(handler.ProjectsHandler)

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

	mux.Handle(
		"/me",
		middleware.AuthMiddleware(http.HandlerFunc(authHandler.MeHandler)),
	)

	mux.Handle(
		"/auth/password",
		middleware.AuthMiddleware(http.HandlerFunc(authHandler.PasswordChangeHandler)),
	)

	mux.Handle(
		"/auth/sessions",
		middleware.AuthMiddleware(http.HandlerFunc(authHandler.SessionsHandler)),
	)

	mux.Handle(
		"/auth/sessions/{id}",
		middleware.AuthMiddleware(http.HandlerFunc(authHandler.DeleteSessionHandler)),
	)

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/admin/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.ListUsersHandler(w, r)
		case http.MethodPost:
			authHandler.CreateUserHandler(w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	adminMux.HandleFunc("/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.GetUserHandler(w, r)
		case http.MethodPut:
			authHandler.UpdateUserHandler(w, r)
		case http.MethodDelete:
			authHandler.DeleteUserHandler(w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	adminHandler := middleware.AuthMiddleware(
		middleware.RequireRole("admin", auditLogger)(adminMux),
	)
	mux.Handle("/admin/", adminHandler)

	cleanupInterval := 24 * time.Hour
	if envInterval := os.Getenv("CLEANUP_INTERVAL"); envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval); err == nil {
			cleanupInterval = parsed
		} else {
			log.Printf("Invalid CLEANUP_INTERVAL '%s', defaulting to 24h: %v", envInterval, err)
		}
	}

	workerDone := make(chan struct{})
	cleanupWorker := worker.NewCleanupWorker(sqlStore, cleanupInterval, workerDone)
	cleanupWorker.Start()

	requestLogger := log.New(os.Stdout, "", log.LstdFlags)
	loggedMux := middleware.RequestLogger(requestLogger)(mux)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: loggedMux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Println("Server running on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	stop()
	close(workerDone)
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly")
}
