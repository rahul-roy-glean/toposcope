// Command toposcoped is the Toposcope platform service.
// It serves the GitHub webhook endpoint, the internal processing endpoint,
// and a health check.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"

	"github.com/toposcope/toposcope/internal/ingestion"
	"github.com/toposcope/toposcope/internal/tenant"
	"github.com/toposcope/toposcope/internal/webhook"
)

type config struct {
	Port           string
	DatabaseURL    string
	GCSBucket      string
	WebhookSecret  string
	GitHubAppID    string
	GitHubKey      string
}

func loadConfig() config {
	return config{
		Port:          envOrDefault("PORT", "8080"),
		DatabaseURL:   envOrDefault("DATABASE_URL", "postgres://localhost:5432/toposcope?sslmode=disable"),
		GCSBucket:     os.Getenv("GCS_BUCKET"),
		WebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		GitHubAppID:   os.Getenv("GITHUB_APP_ID"),
		GitHubKey:     os.Getenv("GITHUB_PRIVATE_KEY"),
	}
}

func main() {
	cfg := loadConfig()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	// Initialize services
	tenantSvc := tenant.NewService(db)

	storagePath := envOrDefault("LOCAL_STORAGE_PATH", "/tmp/toposcope-data")
	storage := ingestion.NewLocalStorage(storagePath)

	ingestionSvc := ingestion.NewService(db, tenantSvc, storage, nil, nil)

	webhookHandler := webhook.NewHandler([]byte(cfg.WebhookSecret), tenantSvc, ingestionSvc)

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.Handle("POST /v1/webhooks/github", webhookHandler)
	mux.HandleFunc("POST /internal/process", processHandler(ingestionSvc))
	mux.HandleFunc("GET /healthz", healthHandler(db))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: mux,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("starting toposcoped on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func processHandler(svc *ingestion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ingestion.IngestionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := svc.ProcessPR(r.Context(), req); err != nil {
			log.Printf("process error: %v", err)
			http.Error(w, "processing failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
	}
}

func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
