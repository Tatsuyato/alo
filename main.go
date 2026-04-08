package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	db, err := openDB(getEnv("DB_PATH", "./alo.db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "db init failed:", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		fmt.Fprintln(os.Stderr, "db migration failed:", err)
		os.Exit(1)
	}

	auth := NewAuthService()
	queue := NewQueue(2)
	defer queue.Stop()

	app := &App{
		db:    db,
		auth:  auth,
		queue: queue,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", app.healthHandler)
	mux.Handle("/api/v1/machine/status", app.withAuth(app.withRateLimit(http.HandlerFunc(app.machineStatusHandler), 15, 30), PermReadStatus))
	mux.Handle("/api/v1/projects", app.withAuth(app.withRateLimit(http.HandlerFunc(app.projectsHandler), 10, 20), PermManageProject))
	mux.Handle("/api/v1/services", app.withAuth(app.withRateLimit(http.HandlerFunc(app.servicesHandler), 10, 20), PermManageProject))
	mux.Handle("/api/v1/commands", app.withAuth(app.withRateLimit(http.HandlerFunc(app.commandsHandler), 10, 20), PermExecuteCommand))
	mux.Handle("/api/v1/builds", app.withAuth(app.withRateLimit(http.HandlerFunc(app.buildsHandler), 6, 12), PermBuildDeploy))
	mux.Handle("/api/v1/jobs/", app.withAuth(app.withRateLimit(http.HandlerFunc(app.jobByIDHandler), 20, 40), PermReadStatus))
	mux.Handle("/api/v1/logs", app.withAuth(app.withRateLimit(http.HandlerFunc(app.logsHandler), 6, 12), PermReadLogs))
	mux.Handle("/api/v1/status", app.withAuth(app.withRateLimit(http.HandlerFunc(app.k8sStatusHandler), 8, 16), PermReadStatus))

	port := strings.TrimSpace(getEnv("PORT", "8080"))
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	fmt.Println("backend api listening on", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "server failed:", err)
		os.Exit(1)
	}
}

type App struct {
	db    *sql.DB
	auth  *AuthService
	queue *Queue
}
