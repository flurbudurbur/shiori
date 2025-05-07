package http

import (
	"context"
	"encoding/json"
	"errors" // Use standard errors package
	"net/http"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/user" // Import user service package for error types
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/rs/zerolog"
	"gorm.io/gorm" // Import gorm for ErrRecordNotFound
)

// authService interface matching the required methods from user.Service
type authService interface {
	GetUserCount(ctx context.Context) (int, error)
	// GenerateUserBookmark (conceptually our RegisterNewUser) now returns the HashedUUID (bookmark string)
	GenerateUserBookmark(ctx context.Context) (hashedUUID string, err error)
	// AuthenticateUser uses the HashedUUID to fetch and authenticate a user.
	// This maps to user.Service.GetUserForAuthentication.
	AuthenticateUser(ctx context.Context, hashedUUID string) (*domain.User, error)
}

type authHandler struct {
	log     zerolog.Logger
	encoder encoder
	config  *domain.Config
	service authService // Use the local interface definition

	cookieStore *sessions.CookieStore
}

// newAuthHandler constructor uses the local authService interface
func newAuthHandler(encoder encoder, log zerolog.Logger, config *domain.Config, cookieStore *sessions.CookieStore, service authService) *authHandler {
	return &authHandler{
		log:         log,
		encoder:     encoder,
		config:      config,
		service:     service,
		cookieStore: cookieStore,
	}
}

func (h authHandler) Routes(r chi.Router) {
	r.Post("/login", h.login)
	r.Post("/logout", h.logout)
	r.Post("/register", h.register)                 // Use bookmark generation as registration
	r.Post("/login-uuid", h.loginWithUUID)          // New endpoint for UUID login
	r.Get("/register/status", h.registrationStatus) // Renamed from canOnboard
	r.Get("/validate", h.validate)
	r.Post("/generate-bookmark", h.register) // Add route for bookmark generation, which also logs in
}

// loginRequest defines the expected JSON body for the login endpoint
type loginRequest struct {
	UUID string `json:"uuid"`
}

// loginResponse defines the JSON response for successful login
type loginResponse struct {
	User  *domain.User `json:"user"`  // Include user details on login
	Token string       `json:"token"` // Add token field for frontend compatibility
}

func (h authHandler) login(w http.ResponseWriter, r *http.Request) {
	var (
		ctx  = r.Context()
		data loginRequest
	)

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.log.Warn().Err(err).Msg("Auth: Failed to decode login request body")
		h.encoder.StatusResponse(ctx, w, nil, http.StatusBadRequest)
		return
	}

	if data.UUID == "" {
		h.log.Warn().Msg("Auth: Login attempt with empty UUID")
		h.encoder.StatusResponse(ctx, w, nil, http.StatusBadRequest)
		return
	}

	h.cookieStore.Options.HttpOnly = true
	h.cookieStore.Options.SameSite = http.SameSiteLaxMode
	h.cookieStore.Options.Path = h.config.Server.BaseURL // Access via nested Server struct

	fwdProto := r.Header.Get("X-Forwarded-Proto")
	if fwdProto == "https" {
		h.cookieStore.Options.Secure = true
		h.cookieStore.Options.SameSite = http.SameSiteStrictMode
	}

	session, _ := h.cookieStore.Get(r, "user_session")

	// AuthenticateUser now expects HashedUUID, which is the UUID from the request
	authenticatedUser, err := h.service.AuthenticateUser(ctx, data.UUID)
	if err != nil {
		uuidPrefix := data.UUID
		if len(uuidPrefix) > 8 {
			uuidPrefix = uuidPrefix[:8]
		}
		h.log.Warn().Err(err).Msgf("Auth: Failed login attempt uuid_prefix: [%s] ip: %s", uuidPrefix, ReadUserIP(r))

		if errors.Is(err, user.ErrAuthenticationFailed) || errors.Is(err, gorm.ErrRecordNotFound) { // Use gorm.ErrRecordNotFound
			h.encoder.StatusResponse(ctx, w, nil, http.StatusUnauthorized)
		} else if errors.Is(err, user.ErrUserExpired) {
			h.encoder.StatusResponse(ctx, w, nil, http.StatusUnauthorized)
		} else {
			h.encoder.StatusResponse(ctx, w, nil, http.StatusInternalServerError)
		}
		return
	}
	if authenticatedUser == nil { // Handle case where user is not found but no error (e.g. GetUserForAuthentication returns nil, nil)
		h.log.Warn().Msgf("Auth: Login attempt for non-existent UUID: [%s]", data.UUID)
		h.encoder.StatusResponse(ctx, w, nil, http.StatusUnauthorized)
		return
	}

	// Set user as authenticated
	session.Values["authenticated"] = true
	session.Values["user_uuid"] = authenticatedUser.HashedUUID // Use HashedUUID
	session.Save(r, w)

	h.encoder.StatusResponse(ctx, w, loginResponse{
		User:  authenticatedUser,
		Token: authenticatedUser.HashedUUID, // Use HashedUUID as the token
	}, http.StatusOK)
}

func (h authHandler) loginWithUUID(w http.ResponseWriter, r *http.Request) {
	var (
		ctx  = r.Context()
		data loginRequest
	)

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.log.Warn().Err(err).Msg("Auth: Failed to decode login-uuid request body")
		h.encoder.StatusResponse(ctx, w, nil, http.StatusBadRequest)
		return
	}

	if data.UUID == "" {
		h.log.Warn().Msg("Auth: Login-uuid attempt with empty UUID")
		h.encoder.StatusResponse(ctx, w, nil, http.StatusBadRequest)
		return
	}

	h.cookieStore.Options.HttpOnly = true
	h.cookieStore.Options.SameSite = http.SameSiteLaxMode
	h.cookieStore.Options.Path = h.config.Server.BaseURL

	fwdProto := r.Header.Get("X-Forwarded-Proto")
	if fwdProto == "https" {
		h.cookieStore.Options.Secure = true
		h.cookieStore.Options.SameSite = http.SameSiteStrictMode
	}

	session, _ := h.cookieStore.Get(r, "user_session")

	// AuthenticateUser now expects HashedUUID
	authenticatedUser, err := h.service.AuthenticateUser(ctx, data.UUID)
	if err != nil {
		uuidPrefix := data.UUID
		if len(uuidPrefix) > 8 {
			uuidPrefix = uuidPrefix[:8]
		}
		h.log.Warn().Err(err).Msgf("Auth: Failed login-uuid attempt uuid_prefix: [%s] ip: %s", uuidPrefix, ReadUserIP(r))

		if errors.Is(err, user.ErrAuthenticationFailed) || errors.Is(err, gorm.ErrRecordNotFound) { // Use gorm.ErrRecordNotFound
			h.encoder.StatusResponse(ctx, w, nil, http.StatusUnauthorized)
		} else if errors.Is(err, user.ErrUserExpired) {
			h.encoder.StatusResponse(ctx, w, nil, http.StatusUnauthorized)
		} else {
			h.encoder.StatusResponse(ctx, w, nil, http.StatusInternalServerError)
		}
		return
	}
	if authenticatedUser == nil { // Handle case where user is not found but no error
		h.log.Warn().Msgf("Auth: Login-uuid attempt for non-existent UUID: [%s]", data.UUID)
		h.encoder.StatusResponse(ctx, w, nil, http.StatusUnauthorized) // Or 404 Not Found
		return
	}

	// Set user as authenticated
	session.Values["authenticated"] = true
	session.Values["user_uuid"] = authenticatedUser.HashedUUID // Use HashedUUID
	session.Save(r, w)

	// Return user information and token upon successful login
	// Use the HashedUUID as the token since that's what the frontend expects
	h.encoder.StatusResponse(ctx, w, loginResponse{
		User:  authenticatedUser,
		Token: authenticatedUser.HashedUUID, // Use HashedUUID as the token
	}, http.StatusOK)
}

func (h authHandler) logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, _ := h.cookieStore.Get(r, "user_session")

	// Revoke users authentication
	session.Values["authenticated"] = false
	delete(session.Values, "user_uuid") // Remove UUID from session
	session.Save(r, w)

	h.encoder.StatusResponse(ctx, w, nil, http.StatusNoContent)
}

// registrationStatus checks if registration is generally possible (it always is now).
func (h authHandler) registrationStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Since multiple users (UUIDs) are allowed, registration is always possible.
	// The old check for userCount > 0 is removed.
	h.encoder.StatusResponse(ctx, w, nil, http.StatusNoContent)
}

func (h authHandler) validate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, _ := h.cookieStore.Get(r, "user_session")

	// Check if user is authenticated
	if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
		http.Error(w, "Forbidden", http.StatusUnauthorized)
		return
	}

	// send empty response as ok
	h.encoder.StatusResponse(ctx, w, nil, http.StatusNoContent)
}

func ReadUserIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}

// generateBookmarkResponse defines the JSON response for the bookmark generation endpoint
type generateBookmarkResponse struct {
	UUID  string       `json:"uuid"`
	User  *domain.User `json:"user,omitempty"`  // Include user details for auto-login
	Token string       `json:"token,omitempty"` // Include token for frontend compatibility
}

// register handles requests to generate a new user bookmark UUID and auto-authenticates.
func (h authHandler) register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Call the service method to generate the UUID
	generatedUUID, err := h.service.GenerateUserBookmark(ctx)
	if err != nil {
		h.log.Error().Err(err).Msg("Auth: Failed to generate user bookmark UUID")
		h.encoder.StatusResponse(ctx, w, map[string]string{"error": "Failed to generate bookmark"}, http.StatusInternalServerError)
		return
	}

	// Automatically authenticate the user with the new UUID
	authenticatedUser, err := h.service.AuthenticateUser(ctx, generatedUUID)
	if err != nil {
		// This should ideally not happen if GenerateUserBookmark creates a valid user
		h.log.Error().Err(err).Str("uuid", generatedUUID).Msg("Auth: Failed to auto-authenticate user after bookmark generation")
		// Still return the UUID, but log the auth error. Frontend might need to retry login.
		h.encoder.StatusResponse(ctx, w, generateBookmarkResponse{
			UUID: generatedUUID,
			// No Token field since authentication failed
		}, http.StatusOK) // Or an error?
		return
	}

	// Set up session for the auto-authenticated user
	h.cookieStore.Options.HttpOnly = true
	h.cookieStore.Options.SameSite = http.SameSiteLaxMode
	h.cookieStore.Options.Path = h.config.Server.BaseURL

	fwdProto := r.Header.Get("X-Forwarded-Proto")
	if fwdProto == "https" {
		h.cookieStore.Options.Secure = true
		h.cookieStore.Options.SameSite = http.SameSiteStrictMode
	}

	session, _ := h.cookieStore.Get(r, "user_session")
	session.Values["authenticated"] = true
	session.Values["user_uuid"] = authenticatedUser.HashedUUID // Use HashedUUID
	err = session.Save(r, w)
	if err != nil {
		h.log.Error().Err(err).Msg("Auth: Failed to save session during registration/auto-login")
		// Even if session save fails, we should still inform the user about the UUID.
		// The client can then attempt to login manually.
		h.encoder.StatusResponse(ctx, w, generateBookmarkResponse{UUID: generatedUUID}, http.StatusOK)
		return
	}

	h.log.Info().Str("uuid", generatedUUID).Msg("Auth: Successfully generated user bookmark UUID and auto-authenticated")
	response := generateBookmarkResponse{
		UUID:  generatedUUID,
		User:  authenticatedUser,
		Token: authenticatedUser.HashedUUID, // Add token for consistency
	}
	h.encoder.StatusResponse(ctx, w, response, http.StatusOK)
}
