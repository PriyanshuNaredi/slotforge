package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Embed the tz database so time.LoadLocation works on hosts without
	// zoneinfo (scratch containers, minimal Windows installs).
	_ "time/tzdata"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/priyanshunaredi/slotforge/internal/auth"
	"github.com/priyanshunaredi/slotforge/internal/bookings"
	"github.com/priyanshunaredi/slotforge/internal/cache"
	"github.com/priyanshunaredi/slotforge/internal/config"
	"github.com/priyanshunaredi/slotforge/internal/database"
	"github.com/priyanshunaredi/slotforge/internal/health"
	"github.com/priyanshunaredi/slotforge/internal/logging"
	"github.com/priyanshunaredi/slotforge/internal/notifications"
	"github.com/priyanshunaredi/slotforge/internal/providers"
)

func main() {
	// Load local environment variables.
	// Production environments should inject these directly.
	_ = godotenv.Load()

	logger := logging.New()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", slog.Any("error", err))
		os.Exit(1)
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
		logger.Error("database initialization failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize Redis.
	redisClient := cache.NewRedisClient(cfg.RedisAddr)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn("redis client shutdown error", slog.Any("error", err))
		}
	}()

	redisCtx, redisCancel := context.WithTimeout(
		context.Background(),
		2*time.Second,
	)

	if err := redisClient.Ping(redisCtx).Err(); err != nil {
		logger.Warn("redis unavailable; continuing without redis; cache operations fail open",
			slog.Any("error", err))
	} else {
		logger.Info("redis connection established")
	}

	redisCancel()

	// Initialize authentication module.
	tokenManager, err := auth.NewTokenManager(
		cfg.JWTSecret,
		cfg.JWTAccessTTL,
		cfg.RefreshTokenTTL,
	)
	if err != nil {
		logger.Error("token manager initialization failed", slog.Any("error", err))
		os.Exit(1)
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
	router.Use(logging.RequestID(), logging.SecurityHeaders(), logging.Access(logger), logging.Recovery(logger))

	// Health endpoints. Redis is optional (fail-open), so readiness only
	// reports its state and never fails because of it.
	health.RegisterRoutes(router, pool, func(ctx context.Context) error {
		return redisClient.Ping(ctx).Err()
	})
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
		logger.Info("api listening", slog.String("port", cfg.Port))

		if err := server.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", slog.Any("error", err))
			os.Exit(1)
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
			logger.Error("email worker configuration error", slog.Any("error", err))
			os.Exit(1)
		}

		workerStarted = true
		emailWorker.SetLogger(logger)

		go func() {
			defer close(workerDone)
			emailWorker.Run(stopCtx)
		}()

		logger.Info("email delivery worker started",
			slog.Int("batch_size", cfg.EmailWorkerBatchSize),
			slog.String("poll_interval", cfg.EmailWorkerPollInterval.String()),
		)
	} else if !cfg.EmailWorkerEnabled {
		logger.Info("email delivery disabled", slog.String("reason", "EMAIL_WORKER_ENABLED is false"))
	} else {
		logger.Info("email delivery disabled",
			slog.String("reason", "RESEND_API_KEY and EMAIL_FROM are not configured"))
	}

	<-stopCtx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("graceful shutdown failed", slog.Any("error", err))
	}

	// stopCtx is cancelled as soon as the shutdown signal arrives, so the
	// worker stops promptly: in-flight sends are cancelled and claimed but
	// unfinished jobs are left for stale-claim recovery. This wait only
	// gives the worker time to observe the cancellation and exit.
	if workerStarted {
		select {
		case <-workerDone:
		case <-time.After(5 * time.Second):
			logger.Warn("email worker did not stop in time")
		}
	}
}
