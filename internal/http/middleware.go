package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	userService "github.com/flurbudurbur/Shiori/internal/user" // Import user service for errors
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// ContextKey is a type for context keys to avoid collisions.
type ContextKey string

const (
	// UserContextKey is the key for storing user information in the context.
	UserContextKey ContextKey = "user"

	// Default rate limit key prefix in Valkey
	rateLimitKeyPrefix = "rate_limit:"
)

// IsAuthenticated checks if a user is authenticated via session.
// This is typically used for web UI authentication.
func (s *Server) IsAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check session
		session, _ := s.cookieStore.Get(r, "user_session") // Error ignored as per original, consider logging

		// Check if user is authenticated
		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuthenticateAPIToken creates a middleware for API token authentication.
// It expects a Bearer token in the Authorization header and validates it against
// the APITokenHash stored for users using bcrypt comparison.
// This middleware implements an INEFFICIENT iterative user lookup as a temporary measure.
func (s *Server) AuthenticateAPIToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := s.log.With().Str("middleware", "AuthenticateAPIToken").Logger()

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger.Debug().Msg("Authorization header missing, denying access.")
			http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			logger.Debug().Msg("Authorization header format must be Bearer {token}")
			http.Error(w, "Unauthorized: Invalid Authorization header format", http.StatusUnauthorized)
			return
		}
		plainToken := parts[1]

		// Authenticate using the user service
		// Note: The user.Service's AuthenticateUserByToken still contains the inefficient iteration warning.
		// This change moves the call to the service layer, but the underlying inefficiency remains until
		// user.Service.AuthenticateUserByToken is refactored.
		authenticatedUser, err := s.userService.AuthenticateUserByToken(r.Context(), plainToken)
		if err != nil {
			// Log the specific error from the service
			logger.Warn().Err(err).Msg("API token authentication failed")

			// Handle specific authentication errors from user service
			if strings.Contains(err.Error(), userService.ErrAuthenticationFailed.Error()) { // Check underlying error string
				http.Error(w, "Unauthorized: Invalid API token", http.StatusUnauthorized)
			} else if strings.Contains(err.Error(), userService.ErrUserExpired.Error()) {
				http.Error(w, "Unauthorized: User account expired", http.StatusUnauthorized)
			} else if strings.Contains(err.Error(), userService.ErrUserLockedOut.Error()) {
				http.Error(w, "Unauthorized: Account locked", http.StatusForbidden) // Or StatusUnauthorized
			} else {
				// For other errors (e.g., database issues during auth), return a generic internal server error
				http.Error(w, "Internal Server Error during authentication", http.StatusInternalServerError)
			}
			return
		}

		// If authenticatedUser is nil and no error, it implies token was not found by the service logic
		// (though current service AuthenticateUserByToken returns ErrAuthenticationFailed in this case)
		if authenticatedUser == nil {
			logger.Debug().Msg("API token authentication returned no user without explicit error.")
			http.Error(w, "Unauthorized: Invalid API token", http.StatusUnauthorized)
			return
		}

		// Note: The user expiry check is now expected to be handled within s.userService.AuthenticateUserByToken
		// If it's not, it should be added there or re-added here.
		// For example, if AuthenticateUserByToken returns a user even if expired:
		// if !authenticatedUser.DeletionDate.IsZero() && authenticatedUser.DeletionDate.Before(time.Now().UTC()) {
		// 	logger.Info().Str("user_hashed_uuid", authenticatedUser.HashedUUID).Time("deletion_date", authenticatedUser.DeletionDate).Msg("User account authenticated but is expired")
		// 	http.Error(w, "Unauthorized: User account expired", http.StatusUnauthorized)
		// 	return
		// }

		logger.Info().Str("user_hashed_uuid", authenticatedUser.HashedUUID).Msg("User successfully authenticated via API token")

		// Store user information in the request context.
		// Consider storing only essential info (e.g., HashedUUID, Scopes) if the full User object is large or contains sensitive data not needed by handlers.
		ctx := context.WithValue(r.Context(), UserContextKey, authenticatedUser)

		// Placeholder for Scope Enforcement:
		// TODO: Implement robust scope enforcement based on authenticatedUser.Scopes.
		// This check should occur *after* successful authentication.
		// Example:
		// requiredScope := getRequiredScopeForHandler(r) // Hypothetical: determines scope needed for r.URL.Path & r.Method
		// if !userHasScope(authenticatedUser.Scopes, requiredScope) { // Hypothetical: checks if requiredScope is in authenticatedUser.Scopes (which is JSON string)
		//     logger.Warn().
		//         Str("user_hashed_uuid", authenticatedUser.HashedUUID).
		//         Str("required_scope", requiredScope).
		//         Str("user_scopes", authenticatedUser.Scopes).
		//         Msg("User lacks required scope for the requested resource")
		//     http.Error(w, "Forbidden: Insufficient scope", http.StatusForbidden)
		//     return
		// }

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggerMiddleware provides structured logging for HTTP requests.
func LoggerMiddleware(logger *zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Create a child logger for this specific request, inheriting parent logger's fields.
			// This allows adding request-specific fields without affecting the global logger.
			reqLogger := logger.With().Logger()
			// Optionally, add initial request-specific fields here if known early.
			// For example, if X-Request-ID is expected from an upstream proxy:
			// if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
			//    reqLogger = reqLogger.With().Str("upstream_request_id", reqID).Logger()
			// }

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				reqID := middleware.GetReqID(r.Context()) // Get request ID from chi middleware

				// Recover and record stack traces in case of a panic
				if rec := recover(); rec != nil {
					reqLogger.Error().
						Str("type", "error").
						Timestamp().
						Interface("recover_info", rec).
						Bytes("debug_stack", debug.Stack()). // Capture stack trace
						Str("request_id", reqID).            // Log request_id with panic
						Msg("Unhandled panic recovered by middleware")
					http.Error(ww, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()

			// Make the request-specific logger available in the context if needed by handlers.
			// Also, ensure chi's log entry (if any) is also passed down.
			// ctxWithLog := context.WithValue(r.Context(), middleware.LogEntryCtxKey, reqLogger)
			// next.ServeHTTP(ww, r.WithContext(ctxWithLog))
			// Simpler: chi's middleware.Logger already injects a log entry. If we want our own, we can.
			// For now, just proceed. If handlers need the logger, they can get it from context if injected.
			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}

// RateLimiter creates a middleware for rate limiting requests based on user ID or IP address.
// It uses a sliding window counter algorithm with Valkey for storing rate limit counters.
func (s *Server) RateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip rate limiting if disabled in config
		if !s.config.Config.RateLimit.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		logger := s.log.With().Str("middleware", "RateLimiter").Logger()

		// Get client identifier (user ID or IP address)
		identifier, identifierType := s.getClientIdentifier(r)

		// Check if client is exempt from rate limiting
		if s.isExemptFromRateLimit(r, identifier, identifierType) {
			logger.Debug().
				Str("identifier", identifier).
				Str("type", identifierType).
				Msg("Client exempt from rate limiting")
			next.ServeHTTP(w, r)
			return
		}

		// Get rate limit configuration
		requestsPerMinute := s.config.Config.RateLimit.RequestsPerMinute
		windowSeconds := s.config.Config.RateLimit.WindowSeconds

		// Default values if not configured
		if requestsPerMinute <= 0 {
			requestsPerMinute = 20 // Default to 20 requests per minute
		}
		if windowSeconds <= 0 {
			windowSeconds = 60 // Default to 60 seconds (1 minute)
		}

		// Check if client has exceeded rate limit
		exceeded, currentCount, err := s.checkRateLimit(r.Context(), identifier, identifierType, requestsPerMinute, windowSeconds)
		if err != nil {
			logger.Error().Err(err).
				Str("identifier", identifier).
				Str("type", identifierType).
				Msg("Error checking rate limit")
			// Allow request to proceed on error to avoid blocking legitimate traffic
			next.ServeHTTP(w, r)
			return
		}

		// If rate limit exceeded, return 429 Too Many Requests
		if exceeded {
			logger.Warn().
				Str("identifier", identifier).
				Str("type", identifierType).
				Int("current_count", currentCount).
				Int("limit", requestsPerMinute).
				Int("window_seconds", windowSeconds).
				Msg("Rate limit exceeded")

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", windowSeconds))

			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		// Add rate limit headers to response
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", requestsPerMinute))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", requestsPerMinute-currentCount))

		logger.Debug().
			Str("identifier", identifier).
			Str("type", identifierType).
			Int("current_count", currentCount).
			Int("limit", requestsPerMinute).
			Msg("Rate limit check passed")

		next.ServeHTTP(w, r)
	})
}

// getClientIdentifier returns the client identifier for rate limiting.
// It tries to use the authenticated user ID first, then falls back to IP address.
func (s *Server) getClientIdentifier(r *http.Request) (string, string) {
	// Try to get user ID from context
	if user, ok := r.Context().Value(UserContextKey).(*domain.User); ok && user != nil {
		return user.HashedUUID, "user_id"
	}

	// Fall back to IP address
	ip := getClientIP(r)
	return ip, "ip_address"
}

// getClientIP extracts the client IP address from the request.
// It handles various headers that might contain the real client IP when behind proxies.
func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// Check for X-Real-IP header
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}

	// Get IP from RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If error, just return RemoteAddr as is
		return r.RemoteAddr
	}

	return ip
}

// isExemptFromRateLimit checks if the client is exempt from rate limiting.
// Exemptions can be based on role or internal IP address.
func (s *Server) isExemptFromRateLimit(r *http.Request, identifier string, identifierType string) bool {
	// Check for exempt roles if identifier is a user ID
	if identifierType == "user_id" {
		if user, ok := r.Context().Value(UserContextKey).(*domain.User); ok && user != nil {
			// Check if user has an exempt role
			// This is a simplified check - in a real system, you'd check against actual roles
			exemptRoles := strings.Split(s.config.Config.RateLimit.ExemptRoles, ",")
			for _, role := range exemptRoles {
				role = strings.TrimSpace(role)
				if role != "" && strings.Contains(user.Scopes, role) {
					return true
				}
			}
		}
	}

	// Check for exempt internal IPs if identifier is an IP address
	if identifierType == "ip_address" {
		exemptIPs := strings.Split(s.config.Config.RateLimit.ExemptInternalIPs, ",")
		for _, exemptIP := range exemptIPs {
			exemptIP = strings.TrimSpace(exemptIP)
			if exemptIP != "" && exemptIP == identifier {
				return true
			}
		}
	}

	return false
}

// checkRateLimit checks if the client has exceeded the rate limit.
// It uses a sliding window counter algorithm with Valkey for storing rate limit counters.
func (s *Server) checkRateLimit(ctx context.Context, identifier string, identifierType string, limit int, windowSeconds int) (bool, int, error) {
	// Get Valkey client
	var valkeyClient = s.valkeyService.GetClient() // This returns a valkeygo.Client
	if valkeyClient == nil {
		return false, 0, fmt.Errorf("valkey client not available")
	}

	// Create a key for the rate limit counter
	key := fmt.Sprintf("%s%s:%s", rateLimitKeyPrefix, identifierType, identifier)

	// Current timestamp
	now := time.Now().Unix()

	// Remove counts older than the window
	cutoff := now - int64(windowSeconds)

	// Remove expired entries (sliding window)
	valkeyClient.Do(ctx, valkeyClient.B().Zremrangebyscore().Key(key).Min("-inf").Max(fmt.Sprintf("%d", cutoff)).Build())

	// Add current request with current timestamp as score
	valkeyClient.Do(ctx, valkeyClient.B().Zadd().Key(key).ScoreMember().ScoreMember(float64(now), fmt.Sprintf("%d", now)).Build())

	// Set expiration on the key to ensure cleanup
	valkeyClient.Do(ctx, valkeyClient.B().Expire().Key(key).Seconds(int64(windowSeconds)).Build())

	// Count the number of requests in the current window
	countCmd := valkeyClient.Do(ctx, valkeyClient.B().Zcard().Key(key).Build())
	if countCmd.Error() != nil {
		return false, 0, fmt.Errorf("error counting rate limit entries: %w", countCmd.Error())
	}

	// Get the count
	count, err := countCmd.AsInt64()
	if err != nil {
		return false, 0, fmt.Errorf("error parsing rate limit count: %w", err)
	}

	// Check if limit exceeded
	return int(count) > limit, int(count), nil
}
