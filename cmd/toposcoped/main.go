// Command toposcoped is the Toposcope platform service.
// It serves the REST API, optional GitHub webhook endpoint,
// internal processing endpoint, and health checks.
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
	"github.com/toposcope/toposcope/internal/platform"
	"github.com/toposcope/toposcope/internal/tenant"
	"github.com/toposcope/toposcope/internal/webhook"
)

type config struct {
	Port             string
	DatabaseURL      string
	APIKey           string
	CacheSize        int
	StorageBackend   string // local | s3 | gcs
	LocalStoragePath string
	S3Bucket         string
	S3Region         string
	S3Endpoint       string
	GCSBucket        string
	AuthMode         string // none | api-key | oidc-proxy
	AutoMigrate      bool
	MigrateOnly      bool
	WebhookSecret    string
}

func loadConfig() config {
	cacheSize := 20
	if v := os.Getenv("SNAPSHOT_CACHE_SIZE"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			cacheSize = parsed
		}
	}

	return config{
		Port:             envOrDefault("PORT", "8080"),
		DatabaseURL:      envOrDefault("DATABASE_URL", "postgres://localhost:5432/toposcope?sslmode=disable"),
		APIKey:           os.Getenv("API_KEY"),
		CacheSize:        cacheSize,
		StorageBackend:   envOrDefault("STORAGE_BACKEND", "local"),
		LocalStoragePath: envOrDefault("LOCAL_STORAGE_PATH", "/tmp/toposcope-data"),
		S3Bucket:         os.Getenv("S3_BUCKET"),
		S3Region:         os.Getenv("S3_REGION"),
		S3Endpoint:       os.Getenv("S3_ENDPOINT"),
		GCSBucket:        os.Getenv("GCS_BUCKET"),
		AuthMode:         envOrDefault("AUTH_MODE", "api-key"),
		AutoMigrate:      os.Getenv("AUTO_MIGRATE") == "true",
		MigrateOnly:      os.Getenv("MIGRATE_ONLY") == "true",
		WebhookSecret:    os.Getenv("GITHUB_WEBHOOK_SECRET"),
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

	// Run migrations if requested
	if cfg.AutoMigrate || cfg.MigrateOnly {
		log.Println("running database migrations...")
		if err := platform.AutoMigrate(db); err != nil {
			log.Printf("FATAL: auto-migrate: %v", err)
			return
		}
		log.Println("migrations complete")
		if cfg.MigrateOnly {
			log.Println("MIGRATE_ONLY=true, exiting")
			return
		}
	}

	// Initialize storage backend
	storage, err := initStorage(context.Background(), cfg)
	if err != nil {
		log.Printf("FATAL: init storage: %v", err)
		return
	}

	// Initialize services
	tenantSvc := tenant.NewService(db)
	ingestionSvc := ingestion.NewService(db, tenantSvc, storage, nil, nil)

	// Initialize API handler
	cache := api.NewSnapshotCache(cfg.CacheSize)
	apiHandler := api.NewHandler(db, tenantSvc, ingestionSvc, cache)

	// Set up HTTP routes
	mux := http.NewServeMux()

	// Conditionally register webhook handler
	if cfg.WebhookSecret != "" {
		webhookHandler := webhook.NewHandler([]byte(cfg.WebhookSecret), tenantSvc, ingestionSvc)
		mux.Handle("POST /v1/webhooks/github", webhookHandler)
	}

	mux.HandleFunc("POST /internal/process", processHandler(ingestionSvc))
	mux.HandleFunc("GET /healthz", healthHandler(db))
	mux.HandleFunc("GET /health", healthHandler(db))

	// Register API routes
	apiHandler.RegisterRoutes(mux)

	// Apply CORS middleware globally, auth middleware on write endpoints
	authMiddleware := api.WriteAuth(api.AuthMode(cfg.AuthMode), cfg.APIKey)
	handler := api.CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isWrite := (r.Method == "POST" || r.Method == "PATCH" || r.Method == "DELETE") &&
			strings.HasPrefix(r.URL.Path, "/api/")
		if isWrite {
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

func initStorage(ctx context.Context, cfg config) (ingestion.StorageClient, error) {
	switch cfg.StorageBackend {
	case "s3":
		return ingestion.NewS3Storage(ctx, ingestion.S3Config{
			Bucket:    cfg.S3Bucket,
			Region:    cfg.S3Region,
			Endpoint:  cfg.S3Endpoint,
			AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		})
	case "gcs":
		return ingestion.NewGCSStorage(ctx, cfg.GCSBucket)
	default: // "local"
		return ingestion.NewLocalStorage(cfg.LocalStoragePath), nil
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
