package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/priyanshunaredi/slotforge/internal/auth"
	"github.com/priyanshunaredi/slotforge/internal/bookings"
	"github.com/priyanshunaredi/slotforge/internal/cache"
	"github.com/priyanshunaredi/slotforge/internal/config"
	"github.com/priyanshunaredi/slotforge/internal/database"
	"github.com/priyanshunaredi/slotforge/internal/health"
	"github.com/priyanshunaredi/slotforge/internal/notifications"
	"github.com/priyanshunaredi/slotforge/internal/providers"
)

func main() {
	// Load local environment variables.
	// Production environments should inject these directly.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	startupCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := database.NewPostgresPool(
		startupCtx,
		cfg.DatabaseURL,
	)
	if err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}
	defer pool.Close()

	// Initialize Redis.
	redisClient := cache.NewRedisClient(cfg.RedisAddr)
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Printf("Redis client shutdown error: %v", err)
		}
	}()

	redisCtx, redisCancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)

	if err := redisClient.Ping(redisCtx).Err(); err != nil {
		log.Printf("WARNING: Redis unavailable: %v", err)
		log.Println("Continuing without Redis. Cache operations will fail open.")
	} else {
		log.Println("Redis connection established successfully")
	}

	redisCancel()

	// Initialize authentication module.
	tokenManager, err := auth.NewTokenManager(
		cfg.JWTSecret,
		cfg.JWTAccessTTL,
		cfg.RefreshTokenTTL,
	)
	if err != nil {
		log.Fatalf("token manager initialization failed: %v", err)
	}

	authRepo := auth.NewRepository(pool)
	authService := auth.NewService(authRepo, tokenManager)
	authHandler := auth.NewHandler(authService)

	// Initialize booking module.
	bookingRepo := bookings.NewRepository(pool)
	// bookingService := bookings.NewService(bookingRepo)
	bookingService := bookings.NewService(bookingRepo, redisClient)
	bookingHandler := bookings.NewHandler(bookingService)

	notificationRepo := notifications.NewRepository(pool)
	notificationService := notifications.NewService(notificationRepo)
	notificationHandler := notifications.NewHandler(notificationService)

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// Health endpoints.
	health.RegisterRoutes(router, pool)
	providerRepo := providers.NewRepository(pool)
	// providerService := providers.NewService(providerRepo)
	providerService := providers.NewService(providerRepo, redisClient)
	providerHandler := providers.NewHandler(providerService)

	// API routes.
	// api := router.Group("/api/v1")

	// Public authentication endpoints.
	// authHandler.RegisterRoutes(api)

	// Booking routes remain unchanged for now.
	// We will protect them in Phase 5.
	// bookingHandler.RegisterRoutes(api)

	api := router.Group("/api/v1")

	authHandler.RegisterRoutes(api)
	bookingHandler.RegisterPublicRoutes(api)
	providerHandler.RegisterPublicRoutes(api)

	protected := api.Group("")
	protected.Use(auth.AuthenticationMiddleware(tokenManager))

	providerRoutes := protected.Group("")
	providerRoutes.Use(auth.RequireRoles("provider"))
	providerHandler.RegisterRoutes(providerRoutes)

	authHandler.RegisterProtectedRoutes(protected)
	bookingHandler.RegisterRoutes(protected)
	notificationHandler.RegisterRoutes(protected)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("SlotForge API listening on port %s", cfg.Port)

		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	// Graceful shutdown.
	stopCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// Email delivery worker. It runs in the background and is skipped when
	// email credentials are not configured, so the API keeps working without
	// email delivery.
	workerDone := make(chan struct{})
	workerStarted := false

	if cfg.EmailWorkerEnabled && cfg.ResendAPIKey != "" && cfg.EmailFrom != "" {
		emailWorker, err := notifications.NewEmailWorker(
			notifications.NewDeliveryRepository(pool),
			notificationRepo,
			notifications.NewResendSender(cfg.ResendAPIKey, cfg.EmailFrom),
			notifications.EmailWorkerConfig{
				BatchSize:        cfg.EmailWorkerBatchSize,
				PollInterval:     cfg.EmailWorkerPollInterval,
				RecoveryInterval: cfg.EmailWorkerRecoveryInterval,
				StaleAfter:       cfg.EmailWorkerStaleAfter,
			},
		)
		if err != nil {
			log.Fatalf("email worker configuration error: %v", err)
		}

		workerStarted = true

		go func() {
			defer close(workerDone)
			emailWorker.Run(stopCtx)
		}()

		log.Printf(
			"Email delivery worker started (batch=%d, poll=%s)",
			cfg.EmailWorkerBatchSize,
			cfg.EmailWorkerPollInterval,
		)
	} else if !cfg.EmailWorkerEnabled {
		log.Println("Email delivery disabled: EMAIL_WORKER_ENABLED is false")
	} else {
		log.Println(
			"Email delivery disabled: RESEND_API_KEY and EMAIL_FROM are not configured",
		)
	}

	<-stopCtx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}

	// stopCtx is cancelled as soon as the shutdown signal arrives, so the
	// worker stops promptly: in-flight sends are cancelled and claimed but
	// unfinished jobs are left for stale-claim recovery. This wait only
	// gives the worker time to observe the cancellation and exit.
	if workerStarted {
		select {
		case <-workerDone:
		case <-time.After(5 * time.Second):
			log.Println("email worker did not stop in time")
		}
	}
}
