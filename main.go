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
	auth "github.com/ONSdigital/dp-authorisation/v2/authorisation"
	"github.com/gorilla/mux"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("application error: %v", err)
	}
}

func run() error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	// Initialize MongoDB
	db, err := mongo.New(cfg.MongoConfig)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	// Initialize S3 client
	s3Client, err := storage.NewS3(cfg)
	if err != nil {
		return err
	}

	// Initialize CloudFront client
	cfClient, err := storage.NewCloudFront(cfg)
	if err != nil {
		return err
	}

	// Initialize Cloudflare client
	cflarClient, err := storage.NewCloudflare(cfg)
	if err != nil {
		return err
	}

	// Initialize Slack notifier
	slackNotifier := notifications.NewSlackNotifier(&cfg.SlackConfig)

	// Initialize temp storage
	tempStorage, err := temp.NewRedisStorage(context.Background(), cfg)
	if err != nil {
		return err
	}

	// Initialize file validator
	fileValidator := validation.NewFileValidator(
		cfg.MaxUploadSize,
		cfg.AllowedExtensions,
		cfg.AllowedMimeTypes,
	)

	// Initialize auth middleware
	ctx := context.Background()
	authMiddleware, err := auth.NewFeatureFlaggedMiddleware(
		ctx,
		cfg.AuthConfig,
		cfg.AuthConfig.JWTVerificationPublicKeys,
	)
	if err != nil {
		return err
	}

	// Start temp file cleanup scheduler
	cleanupCtx, cancelCleanup := context.WithCancel(context.Background())
	defer cancelCleanup()
	go startCleanupScheduler(
		cleanupCtx,
		tempStorage,
		cfg.TempStorageTimeout,
	)

	// Setup router
	router := mux.NewRouter()
	api.Setup(
		ctx,
		cfg,
		router,
		authMiddleware,
		db,
		s3Client,
		cfClient,
		cflarClient,
		slackNotifier,
		tempStorage,
		fileValidator,
	)

	// Create an HTTP server
	server := &http.Server{
		Addr:         cfg.BindAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Starting server on %s", cfg.BindAddr)
		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Shutting down server...")
	case err := <-serverErr:
		return err
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		cfg.GracefulShutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		return err
	}

	log.Println("Server exited")
	return nil
}

func startCleanupScheduler(
	ctx context.Context,
	tempStorage temp.Storage,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval / 2) // Check twice per timeout period
	defer ticker.Stop()

	log.Printf(
		"Starting temp file cleanup scheduler (interval: %v)",
		interval/2,
	)

	for {
		select {
		case <-ctx.Done():
			log.Println("Cleanup scheduler stopped")
			return
		case <-ticker.C:
			// Cleanup is handled by MongoDB TTL index for database records
			// In-memory storage doesn't persist, so no cleanup needed
			// This is a placeholder for Redis implementation
			log.Println("Cleanup check completed")
		}
	}
}
