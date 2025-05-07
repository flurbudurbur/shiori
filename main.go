package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/asaskevich/EventBus"
	"github.com/flurbudurbur/Shiori/internal/auth"
	"github.com/flurbudurbur/Shiori/internal/config"
	"github.com/flurbudurbur/Shiori/internal/database"
	"github.com/flurbudurbur/Shiori/internal/events"
	"github.com/flurbudurbur/Shiori/internal/http"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/internal/notification"
	"github.com/flurbudurbur/Shiori/internal/scheduler"
	"github.com/flurbudurbur/Shiori/internal/server"
	"github.com/flurbudurbur/Shiori/internal/sync"
	"github.com/flurbudurbur/Shiori/internal/update"
	"github.com/flurbudurbur/Shiori/internal/user"
	"github.com/flurbudurbur/Shiori/internal/valkey"
	"github.com/r3labs/sse/v2"
	"github.com/spf13/pflag"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

// NoOpRateLimiter is a placeholder implementation of user.RateLimiterStore
// that performs no operations. Used to satisfy dependencies during build.
type NoOpRateLimiter struct{}

func (n *NoOpRateLimiter) IsLockedOut(ctx context.Context, uuid string) (bool, error) {
	return false, nil // Never locked out
}

func (n *NoOpRateLimiter) IncrementFailure(ctx context.Context, uuid string) (int64, error) {
	return 0, nil // Always return 0 failures
}

func (n *NoOpRateLimiter) ClearFailures(ctx context.Context, uuid string) error {
	return nil // No-op
}

func (n *NoOpRateLimiter) SetLockout(ctx context.Context, uuid string) error {
	return nil // No-op
}

// Ensure NoOpRateLimiter implements the interface
var _ user.RateLimiterStore = (*NoOpRateLimiter)(nil)

func main() {
	var configPath string
	pflag.StringVar(&configPath, "config", "", "path to configuration file")
	pflag.Parse()

	// read config
	cfg := config.New(configPath, version)

	// init new logger
	log := logger.New(cfg.Config)

	// init dynamic config
	cfg.DynamicReload(log)

	// setup server-sent-events
	serverEvents := sse.New()
	serverEvents.CreateStreamWithOpts("logs", sse.StreamOpts{MaxEntries: 1000, AutoReplay: true})

	// register SSE writer
	log.RegisterSSEWriter(serverEvents)

	// setup internal eventbus
	bus := EventBus.New()

	// open database connection
	db, err := database.NewDB(cfg.Config, log)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create new db")
	}

	if err := db.Open(); err != nil {
		log.Fatal().Err(err).Msg("could not open db connection")
	}

	log.Info().Msgf("Starting SyncYomi")
	log.Info().Msgf("Version: %s", version)
	log.Info().Msgf("Commit: %s", commit)
	log.Info().Msgf("Build date: %s", date)
	log.Info().Msgf("Log-level: %s", cfg.Config.Logging.Level)
	log.Info().Msgf("Using database: %s", db.Driver)

	// setup repos
	var (
		notificationRepo = database.NewNotificationRepo(log, db)
		userRepo         = database.NewUserRepo(log, db)
		syncRepo         = database.NewSyncRepo(log, db)
		profileUUIDRepo  = database.NewProfileUUIDRepo(log, db)
	)

	// setup placeholder rate limiter
	rateLimiter := &NoOpRateLimiter{}

	// init Valkey service
	valkeyService, err := valkey.NewService(cfg.Config.Valkey)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create new valkey service")
	}
	defer valkeyService.Close()
	log.Info().Msg("Valkey service initialized")

	// setup services
	var (
		notificationService = notification.NewService(log, notificationRepo)
		updateService       = update.NewUpdate(log, cfg.Config)
		// Pass userRepo and profileUUIDRepo to scheduler service
		schedulingService = scheduler.NewService(log, cfg.Config, notificationService, updateService, userRepo, profileUUIDRepo)
		// Pass rateLimiter, logger, valkeyService, and profileUUIDRepo to user service
		userService = user.NewService(userRepo, rateLimiter, log, valkeyService, profileUUIDRepo) // Added profileUUIDRepo
		authService = auth.NewService(log, userService)                                           // Instantiate auth service
		syncService = sync.NewService(log, syncRepo, notificationService)
	)

	// register event subscribers
	events.NewSubscribers(log, bus, notificationService)

	errorChannel := make(chan error)

	go func() {
		httpServer := http.NewServer(
			log,
			cfg,
			serverEvents,
			db,
			version,
			commit,
			date,
			// Pass authService which now implements the required http.authService interface
			authService,
			notificationService,
			updateService,
			userService,
			syncService,
			valkeyService, // Pass valkeyService for rate limiting
		)
		errorChannel <- httpServer.Open()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	srv := server.NewServer(log, cfg.Config, schedulingService, updateService)
	if err := srv.Start(); err != nil {
		log.Fatal().Stack().Err(err).Msg("could not start server")
		return
	}

	for sig := range sigCh {
		switch sig {
		case syscall.SIGHUP:
			log.Log().Msg("shutting down server sighup")
			srv.Shutdown()
			err := db.Close()
			if err != nil {
				log.Error().Stack().Err(err).Msg("could not close db connection")
				// Don't return immediately, try to close Valkey as well
			}
			valkeyService.Close()
			log.Info().Msg("Valkey service shut down")
			os.Exit(1)
		case syscall.SIGINT, syscall.SIGQUIT:
			log.Info().Msg("Shutting down server due to SIGINT/SIGQUIT...")
			srv.Shutdown()
			err := db.Close()
			if err != nil {
				log.Error().Stack().Err(err).Msg("could not close db connection")
			}
			valkeyService.Close()
			log.Info().Msg("Valkey service shut down")
			os.Exit(0) // Graceful exit
		case syscall.SIGKILL, syscall.SIGTERM:
			log.Info().Msg("Shutting down server due to SIGKILL/SIGTERM...")
			srv.Shutdown()
			err := db.Close()
			if err != nil {
				log.Error().Stack().Err(err).Msg("could not close db connection")
			}
			valkeyService.Close()
			log.Info().Msg("Valkey service shut down")
			os.Exit(0) // Graceful exit
		}
	}
}
