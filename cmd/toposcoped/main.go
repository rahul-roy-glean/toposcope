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
	"strconv"
	"strings"
	"syscall"

	_ "github.com/lib/pq"

	"github.com/toposcope/toposcope/internal/api"
	"github.com/toposcope/toposcope/internal/ingestion"
	"github.com/toposcope/toposcope/internal/tenant"
	"github.com/toposcope/toposcope/internal/webhook"
)

type config struct {
	Port          string
	DatabaseURL   string
	GCSBucket     string
	WebhookSecret string
	GitHubAppID   string
	GitHubKey     string
	APIKey        string
	CacheSize     int
}

func loadConfig() config {
	cacheSize := 20
	if v := os.Getenv("SNAPSHOT_CACHE_SIZE"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			cacheSize = parsed
		}
	}

	return config{
		Port:          envOrDefault("PORT", "8080"),
		DatabaseURL:   envOrDefault("DATABASE_URL", "postgres://localhost:5432/toposcope?sslmode=disable"),
		GCSBucket:     os.Getenv("GCS_BUCKET"),
		WebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		GitHubAppID:   os.Getenv("GITHUB_APP_ID"),
		GitHubKey:     os.Getenv("GITHUB_PRIVATE_KEY"),
		APIKey:        os.Getenv("API_KEY"),
		CacheSize:     cacheSize,
	}
}

func main() {
	cfg := loadConfig()

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		log.Fatalf("ping database: %v", err)
	}
	defer db.Close()

	// Initialize services
	tenantSvc := tenant.NewService(db)

	storagePath := envOrDefault("LOCAL_STORAGE_PATH", "/tmp/toposcope-data")
	storage := ingestion.NewLocalStorage(storagePath)

	ingestionSvc := ingestion.NewService(db, tenantSvc, storage, nil, nil)

	webhookHandler := webhook.NewHandler([]byte(cfg.WebhookSecret), tenantSvc, ingestionSvc)

	// Initialize API handler
	cache := api.NewSnapshotCache(cfg.CacheSize)
	apiHandler := api.NewHandler(db, tenantSvc, ingestionSvc, cache)

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.Handle("POST /v1/webhooks/github", webhookHandler)
	mux.HandleFunc("POST /internal/process", processHandler(ingestionSvc))
	mux.HandleFunc("GET /healthz", healthHandler(db))

	// Register API routes
	apiHandler.RegisterRoutes(mux)

	// Apply CORS middleware globally, API key auth selectively to /api/ routes
	authMiddleware := api.APIKeyAuth(cfg.APIKey)
	handler := api.CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			authMiddleware(mux).ServeHTTP(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	}))

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
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
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
	}
}

func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.PingContext(r.Context()); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
