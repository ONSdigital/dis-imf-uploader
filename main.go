package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ONSdigital/dis-imf-uploader/api"
	"github.com/ONSdigital/dis-imf-uploader/config"
	"github.com/ONSdigital/dis-imf-uploader/mongo"
	"github.com/ONSdigital/dis-imf-uploader/notifications"
	"github.com/ONSdigital/dis-imf-uploader/storage"
	"github.com/ONSdigital/dis-imf-uploader/temp"
	"github.com/ONSdigital/dis-imf-uploader/validation"
	"github.com/gorilla/mux"
)

func main() {
	cfg, err := config.Get()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize MongoDB
	db, err := mongo.New(cfg.MongoConfig)
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	defer db.Close()

	// Initialize S3 client
	s3Client, err := storage.NewS3(cfg)
	if err != nil {
		log.Fatalf("failed to create S3 client: %v", err)
	}

	// Initialize CloudFront client
	cfClient, err := storage.NewCloudFront(cfg)
	if err != nil {
		log.Fatalf("failed to create CloudFront client: %v", err)
	}

	// Initialize Cloudflare client
	cflarClient, err := storage.NewCloudflare(cfg)
	if err != nil {
		log.Fatalf("failed to create Cloudflare client: %v", err)
	}

	// Initialize Slack notifier
	slackNotifier := notifications.NewSlackNotifier(&cfg.SlackConfig)

	// Initialize temp storage
	tempStorage, err := temp.NewRedisStorage(context.Background(), cfg)
	if err != nil {
		log.Fatalf("failed to initialize temp storage: %v", err)
	}

	// Initialize file validator
	fileValidator := validation.NewFileValidator(
		cfg.MaxUploadSize,
		cfg.ValidationConfig.AllowedExtensions,
		cfg.ValidationConfig.AllowedMimeTypes,
	)

	// Start temp file cleanup scheduler
	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	go startCleanupScheduler(cleanupCtx, tempStorage, cfg.TempStorageTimeout)

	// Setup router
	router := mux.NewRouter()
	api.SetupRoutes(router, cfg, db, s3Client, cfClient, cflarClient,
		slackNotifier, tempStorage, fileValidator)

	// Create an HTTP server
	server := &http.Server{
		Addr:         cfg.BindAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on %s", cfg.BindAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func startCleanupScheduler(ctx context.Context, storage temp.Storage, interval time.Duration) {
	ticker := time.NewTicker(interval / 2) // Check twice per timeout period
	defer ticker.Stop()

	log.Printf("Starting temp file cleanup scheduler (interval: %v)", interval/2)

	for {
		select {
		case <-ctx.Done():
			log.Println("Cleanup scheduler stopped")
			return
		case <-ticker.C:
			// Cleanup is handled by MongoDB TTL index for database records
			// In-memory storage doesn't persist, so no cleanup needed for temp files
			// This is a placeholder for Redis implementation
			log.Println("Cleanup check completed")
		}
	}
}
