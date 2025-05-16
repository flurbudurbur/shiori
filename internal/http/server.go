package http

import (
	"fmt"
	"net"
	"net/http"

	"github.com/flurbudurbur/Shiori/web"
	"github.com/flurbudurbur/Shiori/internal/config"
	"github.com/flurbudurbur/Shiori/internal/database"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
	"github.com/r3labs/sse/v2"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	valkeygo "github.com/valkey-io/valkey-go"

	userservice "github.com/flurbudurbur/Shiori/internal/user" // Import for user.Service with alias
)

// Define service interfaces
type valkeyService interface {
	GetClient() valkeygo.Client
	Close()
}

type Server struct {
	log zerolog.Logger
	sse *sse.Server
	db  *database.DB

	config      *config.AppConfig
	cookieStore *sessions.CookieStore

	version string
	commit  string
	date    string

	authService         authService
	notificationService notificationService
	updateService       updateService
	userService         userservice.Service // Use aliased user.Service
	syncService         syncService
	valkeyService       valkeyService // Valkey service for rate limiting
}

func NewServer(
	log logger.Logger, // This is logger.Logger (interface)
	config *config.AppConfig,
	sse *sse.Server,
	db *database.DB,
	version string,
	commit string,
	date string,
	authService authService,
	notificationSvc notificationService,
	updateSvc updateService,
	userSvc userservice.Service, // Use aliased user.Service
	syncService syncService,
	valkeyService valkeyService, // Valkey service for rate limiting
) Server {
	// The logger passed in is logger.Logger, but s.log is zerolog.Logger.
	// We need to ensure the concrete zerolog.Logger is used for NewUserResource.
	concreteLog := log.With().Str("module", "http").Logger() // This is zerolog.Logger

	return Server{
		log:     concreteLog, // Store the concrete zerolog.Logger
		config:  config,
		sse:     sse,
		db:      db,
		version: version,
		commit:  commit,
		date:    date,

		cookieStore: sessions.NewCookieStore([]byte(config.Config.SessionSecret)),

		authService:         authService,
		notificationService: notificationSvc,
		updateService:       updateSvc,
		userService:         userSvc,
		syncService:         syncService,
		valkeyService:       valkeyService,
	}
}

func (s Server) Open() error {
	addr := fmt.Sprintf("%v:%v", s.config.Config.Server.Host, s.config.Config.Server.Port) // Access via nested Server struct
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	server := http.Server{
		Handler: s.Handler(),
	}

	s.log.Info().Msgf("Starting server. Listening on %s", listener.Addr().String())

	return server.Serve(listener)
}

func (s Server) Handler() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(LoggerMiddleware(&s.log)) // Pass the zerolog.Logger

	c := cors.New(cors.Options{
		AllowCredentials:   true,
		AllowedMethods:     []string{"HEAD", "OPTIONS", "GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowOriginFunc:    func(origin string) bool { return true },
		OptionsPassthrough: true,
		// Enable Debugging for testing, consider disabling in production
		Debug: false,
	})

	r.Use(c.Handler)

	encoder := encoder{}

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", newAuthHandler(encoder, s.log, s.config.Config, s.cookieStore, s.authService).Routes)
		r.Route("/healthz", newHealthHandler(encoder, s.db).Routes)

		// Route for UUID generation - apply rate limiting
		uuidRouter := r.Group(nil)
		uuidRouter.Use(s.RateLimiter) // Apply rate limiting middleware
		uuidRouter.Post("/v1/utils/uuid", s.handleGetUUID)

		// Authenticated routes group
		authedRouter := r.Group(nil)             // Create a new group for authenticated routes
		authedRouter.Use(s.AuthenticateAPIToken) // Apply API token authentication middleware

		// User-specific routes (profile, token management)
		// Pass s.log (which is zerolog.Logger) to NewUserResource
		userResource := NewUserResource(s.userService, s.log, encoder)

		// Create a rate-limited router for profile-related endpoints
		profileRouter := authedRouter.Group(nil)
		profileRouter.Use(s.RateLimiter) // Apply rate limiting middleware

		// Apply rate limiting to profile-related endpoints
		profileRouter.Post("/profile/api-token", userResource.handleResetAndGetUserToken)
		profileRouter.Get("/profile", userResource.handleGetProfile) // New route for getting profile data

		authedRouter.Route("/config", newConfigHandler(encoder, s, s.config).Routes)
		authedRouter.Route("/logs", newLogsHandler(s.config).Routes)
		authedRouter.Route("/notification", newNotificationHandler(encoder, s.notificationService).Routes)
		authedRouter.Route("/updates", newUpdateHandler(encoder, s.updateService).Routes)
		// Apply rate limiting to sync endpoints as they can trigger UUID generation
		syncRouter := authedRouter.Group(nil)
		syncRouter.Use(s.RateLimiter) // Apply rate limiting middleware
		syncRouter.Route("/sync", newSyncHandler(encoder, s.syncService, s.userService).Routes)

		authedRouter.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
			// inject CORS headers to bypass checks
			s.sse.Headers = map[string]string{
				"Content-Type":      "text/event-stream",
				"Cache-Control":     "no-cache",
				"Connection":        "keep-alive",
				"X-Accel-Buffering": "no",
			}
			s.sse.ServeHTTP(w, r)
		})
	})

	// serve the web
	frontend.RegisterHandler(r, s.version, s.config.Config.Server.BaseURL) // Access via nested Server struct

	return r
}
