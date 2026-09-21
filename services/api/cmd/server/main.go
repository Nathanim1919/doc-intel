package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/doc-intel/api/internal/config"
	"github.com/doc-intel/api/internal/db"
	"github.com/doc-intel/api/internal/document"
	"github.com/doc-intel/api/internal/queue"
	"github.com/doc-intel/api/internal/storage"
)

func main() {
	ctx := context.Background()

	// --- Config ---
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// --- Database ---
	pool, err := db.NewPool(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	// --- Object Storage ---
	store, err := storage.NewMinioStorage(
		cfg.MinioEndpoint,
		cfg.MinioAccessKey,
		cfg.MinioSecretKey,
		cfg.MinioBucket,
		cfg.MinioUseSSL,
	)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}
	if err := store.EnsureBucket(ctx); err != nil {
		log.Fatalf("storage: ensure bucket: %v", err)
	}

	// --- Queue ---
	redisClient, err := queue.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("queue: %v", err)
	}
	defer redisClient.Close()
	producer := queue.NewRedisProducer(redisClient)

	// --- Repositories & Service ---
	docRepo := db.NewDocumentRepository(pool)
	jobRepo := db.NewJobRepository(pool)
	docService := document.New(pool, docRepo, jobRepo, store, producer)
	_ = docService // handlers wired in next layer

	// --- HTTP Server ---
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, `{"status":"ok"}`)
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("api: listening on :%d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api: listen: %v", err)
		}
	}()

	<-quit
	log.Println("api: shutting down...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("api: forced shutdown: %v", err)
	}
	log.Println("api: stopped")
}
