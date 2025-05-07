package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	// "database/sql" // No longer needed directly here
	"errors"
	"fmt"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"      // Import logger package
	"github.com/flurbudurbur/Shiori/internal/valkey"      // Import Valkey service
	pkgErrors "github.com/flurbudurbur/Shiori/pkg/errors" // Use alias to avoid conflict
	"github.com/google/uuid"                              // For UUID generation
	valkeyClient "github.com/valkey-io/valkey-go"         // Direct import for ErrNil
	"golang.org/x/crypto/bcrypt"                          // For API Token hashing
	"gorm.io/gorm"                                        // Import gorm for ErrRecordNotFound
)

var (
	ErrAuthenticationFailed = pkgErrors.New("authentication failed")
	ErrUserExpired          = pkgErrors.New("user account expired")
	ErrUserLockedOut        = pkgErrors.New("user account locked out") // Keep for potential future use
	ErrTokenGeneration      = pkgErrors.New("failed to generate API token")
	ErrTokenHashing         = pkgErrors.New("failed to hash API token")
)

// UUIDGenerationError wraps the original error from uuid generation
type UUIDGenerationError struct {
	OriginalError error
}

// Error returns the formatted error message, consistent with previous wrapping.
func (e *UUIDGenerationError) Error() string {
	return fmt.Sprintf("failed to generate UUID: %v", e.OriginalError)
}

// Unwrap allows checking the underlying error type using errors.Is/As.
func (e *UUIDGenerationError) Unwrap() error {
	return e.OriginalError
}

// RateLimiterStore defines the interface for interacting with the rate limiting backend (Redis)
// Kept for potential future use with token auth, but implementation needs review.
type RateLimiterStore interface {
	IsLockedOut(ctx context.Context, key string) (bool, error)       // Key could be HashedUUID or token hash prefix
	IncrementFailure(ctx context.Context, key string) (int64, error) // Returns new count
	ClearFailures(ctx context.Context, key string) error
	SetLockout(ctx context.Context, key string) error
}

// Configurable rate limiting parameters (adjust as needed for token auth)
const (
	lockoutThreshold = 10 // Failures before lockout (adjust for token attempts)
	// lockoutDuration  = 30 * time.Minute // Defined in Redis implementation?
	// failureWindow    = 10 * time.Minute // Defined in Redis implementation?
)

// Default scopes for a new user
const defaultScopes = `{"read": true, "write": false}` // Example JSON string

type Service interface {
	GetUserCount(ctx context.Context) (int, error)
	// RegisterNewUser generates a new HashedUUID, generates/hashes an API token,
	// stores the user, and returns the HashedUUID (bookmark).
	RegisterNewUser(ctx context.Context) (hashedUUID string, err error)
	// AuthenticateUserByToken verifies the provided plain API token against stored hashes.
	// WARNING: Current implementation iterates through users and is inefficient.
	AuthenticateUserByToken(ctx context.Context, plainToken string) (*domain.User, error) // This remains for potential API use
	// GetUserForAuthentication retrieves a user by HashedUUID, intended for use after successful token auth.
	GetUserForAuthentication(ctx context.Context, hashedUUID string) (*domain.User, error)
	// ResetAndRetrieveUserToken generates a new API token for the user, stores its hash, and returns the plain token.
	ResetAndRetrieveUserToken(ctx context.Context, hashedUUID string) (plainToken string, err error)
	// GetOrGenerateProfileUUID retrieves an existing profile-specific UUID from Valkey or the persistent database,
	// or generates a new one if not found.
	GetOrGenerateProfileUUID(ctx context.Context, sessionID string) (string, error)

	// PromoteProfileUUID promotes a profile UUID from Valkey to the persistent database.
	PromoteProfileUUID(ctx context.Context, userID string, profileUUID string) error
}

type service struct {
	repo            domain.UserRepo
	limiter         RateLimiterStore       // Keep limiter, adapt usage
	log             logger.Logger          // Add logger field
	valkeyService   *valkey.Service        // Add Valkey service
	profileUUIDRepo domain.ProfileUUIDRepo // Add ProfileUUID repository
}

// NewService creates a new user service instance.
func NewService(repo domain.UserRepo, limiter RateLimiterStore, log logger.Logger, valkeyService *valkey.Service, profileUUIDRepo domain.ProfileUUIDRepo) Service {
	return &service{
		repo:            repo,
		limiter:         limiter,
		log:             log, // Store the logger interface instance directly
		valkeyService:   valkeyService,
		profileUUIDRepo: profileUUIDRepo,
	}
}

func (s *service) GetUserCount(ctx context.Context) (int, error) {
	count, err := s.repo.GetUserCount(ctx)
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Msg("Failed to get user count from repository")
		// Wrap error for consistent error handling upstream
		return 0, pkgErrors.Wrap(err, "failed to retrieve user count")
	}
	return count, nil
}

// generateApiToken creates a cryptographically secure random token string.
func generateApiToken(byteLength int) (string, error) {
	tokenBytes := make([]byte, byteLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", ErrTokenGeneration
	}
	return hex.EncodeToString(tokenBytes), nil
}

// hashApiToken hashes the token using bcrypt.
func hashApiToken(token string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", ErrTokenHashing
	}
	return string(hashedBytes), nil
}

// generateAndStoreUserToken generates, hashes, and prepares token/scopes for a user.
// It returns the plain token. It does NOT store the user itself.
func (s *service) generateAndStoreUserToken(user *domain.User) (string, error) {
	// 1. Generate plain token
	// TODO: Make token byte length configurable
	plainToken, err := generateApiToken(32) // 32 bytes = 64 hex characters
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Msg("Failed to generate secure API token")
		return "", err // Already wrapped ErrTokenGeneration
	}

	// 2. Hash the token
	hashedToken, err := hashApiToken(plainToken)
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Msg("Failed to hash API token")
		return "", err // Already wrapped ErrTokenHashing
	}

	// 3. Set fields on the user object
	user.APITokenHash = hashedToken
	if user.Scopes == "" { // Set default scopes if none provided
		user.Scopes = defaultScopes
	}

	// 4. Return plain token (caller is responsible for storing the user)
	return plainToken, nil
}

// hashUUID generates a unique identifier for the user record itself (not for auth).
// Using UUID v4 for simplicity.
func generateHashedUUID() (string, error) {
	newUUID, err := uuid.NewRandom()
	if err != nil {
		return "", &UUIDGenerationError{OriginalError: err}
	}
	// In this refactor, the "HashedUUID" is just the UUID string itself.
	// The name is kept for consistency with the previous phase but might be confusing.
	// Consider renaming to UserUUID or similar in a future refactor.
	return newUUID.String(), nil
}

func (s *service) RegisterNewUser(ctx context.Context) (string, error) {
	s.log.Debug().Str("service", "user").Msg("Attempting to register new user")

	// 1. Generate HashedUUID for the user record
	hashedUUID, err := generateHashedUUID()
	if err != nil {
		// Logging handled within generateHashedUUID if it wraps logger
		// If not, log here: s.log.Error().Err(err).Msg("Failed to generate HashedUUID")
		return "", pkgErrors.Wrap(err, "failed to generate user identifier") // Return wrapped error
	}

	// 2. Calculate deletion date
	// TODO: Make deletion duration configurable
	deletionDate := time.Now().AddDate(0, 0, 60) // Now + 60 days

	// 3. Create User object (initially without token)
	newUser := domain.User{
		HashedUUID:   hashedUUID,
		DeletionDate: deletionDate,
		// Scopes will be set by generateAndStoreUserToken if empty
	}

	// 4. Generate and hash API token, set fields on newUser
	_, err = s.generateAndStoreUserToken(&newUser) // plainToken is not returned by RegisterNewUser anymore
	if err != nil {
		// Logging handled within generateAndStoreUserToken
		return "", err // Return error from token generation/hashing
	}

	// 5. Store user with HashedUUID, APITokenHash, Scopes, DeletionDate
	err = s.repo.Store(ctx, newUser)
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Str("hashed_uuid", newUser.HashedUUID).Msg("Failed to store new user in repository")
		// Handle potential unique constraint violations (e.g., duplicate HashedUUID - unlikely, or APITokenHash - very unlikely)
		return "", pkgErrors.Wrap(err, "failed to store new user")
	}

	s.log.Info().Str("service", "user").Str("hashed_uuid", newUser.HashedUUID).Msg("Successfully registered new user")
	// 6. Return HashedUUID (the bookmark)
	return newUser.HashedUUID, nil
}

// AuthenticateUserByToken attempts to authenticate a user using a plain API token.
// WARNING: This implementation iterates through all users and compares hashes,
// which is highly inefficient and not suitable for production with many users.
// A different approach (e.g., caching, different token scheme) is needed.
func (s *service) AuthenticateUserByToken(ctx context.Context, plainToken string) (*domain.User, error) {
	s.log.Debug().Str("service", "user").Msg("Attempting authentication via API token")

	// --- Rate Limiting Check (Example using plainToken prefix) ---
	// Using a hash or prefix of the token might be better than the full token as key.
	// This part needs careful consideration based on security/privacy needs.
	rateLimitKey := plainToken
	if len(rateLimitKey) > 16 { // Use prefix to avoid storing full tokens in Redis keys
		rateLimitKey = rateLimitKey[:16]
	}

	locked, err := s.limiter.IsLockedOut(ctx, rateLimitKey)
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Str("key_prefix", rateLimitKey).Msg("Rate limiter check failed")
		// Fail closed if limiter check fails
		return nil, ErrAuthenticationFailed
	}
	if locked {
		s.log.Warn().Str("service", "user").Str("key_prefix", rateLimitKey).Msg("Authentication attempt rejected due to rate limiting lockout")
		return nil, ErrUserLockedOut
	}

	// --- Inefficient User Iteration ---
	// TODO: Replace this entire block with an efficient lookup method.
	// This requires either changing the token format (e.g., include HashedUUID)
	// or using a different storage/lookup mechanism.
	s.log.Warn().Msg("Performing inefficient user iteration for token authentication. Replace this!")

	// Fetching ALL users is not feasible. This needs a proper solution.
	// For demonstration, let's assume we can somehow get relevant users.
	// Placeholder: In a real scenario, you CANNOT fetch all users.
	// We will simulate failure here as fetching all is wrong.
	// If FindByAPITokenHash existed and worked with bcrypt, we'd use it.
	// Since it doesn't work directly with bcrypt comparison, we simulate the issue.

	// Correct approach would involve repo changes or different token strategy.
	// For now, we cannot implement this correctly without fetching all users.
	// Let's call the repo's FindByAPITokenHash, acknowledging it won't work as intended
	// with bcrypt comparison logic here. The repo method itself might be useful
	// if the *hash* was known, but it isn't from the plain token.

	// Let's pretend we found a user via magic (THIS IS WRONG):
	// magicUser, err := s.repo.FindByAPITokenHash(ctx, "some_hash_we_dont_have")

	// Corrected (but still inefficient) approach: Fetch users page by page? Still bad.
	// Let's just fail here for now, as the premise is flawed without a better lookup.
	s.log.Error().Str("service", "user").Msg("Cannot implement AuthenticateUserByToken efficiently with current structure. Failing authentication.")
	s.handleAuthFailure(ctx, rateLimitKey) // Increment failure count for the token prefix
	return nil, ErrAuthenticationFailed

	/* --- Start of Hypothetical Correct Logic (If User Lookup Was Possible) ---
	var foundUser *domain.User = nil // Assume magicLookupFunction finds the potential user

	if foundUser == nil {
		s.log.Warn().Str("service", "user").Msg("No user found matching the provided token (simulated)")
		s.handleAuthFailure(ctx, rateLimitKey)
		return nil, ErrAuthenticationFailed
	}

	// --- Verification ---
	// 1. Compare provided plain token with the found user's hash
	err = bcrypt.CompareHashAndPassword([]byte(foundUser.APITokenHash), []byte(plainToken))
	if err != nil {
		// If err is bcrypt.ErrMismatchedHashAndPassword, it's a standard auth failure.
		// Other errors might indicate issues with the hash itself.
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			s.log.Warn().Str("service", "user").Str("hashed_uuid", foundUser.HashedUUID).Msg("API token mismatch")
		} else {
			s.log.Error().Str("service", "user").Err(err).Str("hashed_uuid", foundUser.HashedUUID).Msg("Error comparing token hash")
		}
		s.handleAuthFailure(ctx, rateLimitKey) // Use token prefix for unidentified failure
		return nil, ErrAuthenticationFailed
	}

	// 2. Check if expired
	if time.Now().After(foundUser.DeletionDate) {
		s.log.Warn().Str("service", "user").Str("hashed_uuid", foundUser.HashedUUID).Msg("Authentication failed: User account expired")
		// Don't increment failure count for expired users
		return nil, ErrUserExpired
	}

	// --- Success Path ---
	s.log.Info().Str("service", "user").Str("hashed_uuid", foundUser.HashedUUID).Msg("API token authentication successful")

	// 3. Clear any previous failures for this user in Redis (use HashedUUID now we know who it is)
	clearErr := s.limiter.ClearFailures(ctx, foundUser.HashedUUID)
	if clearErr != nil {
		s.log.Error().Str("service", "user").Err(clearErr).Str("hashed_uuid", foundUser.HashedUUID).Msg("Failed to clear rate limiter failures after successful auth")
		// Proceed with successful auth despite limiter error
	}

	// 4. Update deletion date in DB
	newDeletionDate := time.Now().AddDate(0, 0, 60) // TODO: Make duration configurable
	updateErr := s.repo.UpdateDeletionDate(ctx, foundUser.HashedUUID, newDeletionDate)
	if updateErr != nil {
		s.log.Error().Str("service", "user").Err(updateErr).Str("hashed_uuid", foundUser.HashedUUID).Msg("Failed to update user deletion date after successful auth")
		// Proceed with successful auth, but user object won't have updated date
	} else {
		// Update the user object's deletion date only if DB update was successful
		foundUser.DeletionDate = newDeletionDate
	}

	// 5. Return authenticated user
	return foundUser, nil
	--- End of Hypothetical Correct Logic --- */
}

// GetUserForAuthentication retrieves user details by HashedUUID.
// Typically called after successful authentication to get the full user object.
func (s *service) GetUserForAuthentication(ctx context.Context, hashedUUID string) (*domain.User, error) {
	s.log.Debug().Str("service", "user").Str("hashed_uuid", hashedUUID).Msg("Getting user details for authentication context")
	user, err := s.repo.FindByHashedUUID(ctx, hashedUUID)
	if err != nil {
		// Log unexpected errors
		if !errors.Is(err, gorm.ErrRecordNotFound) { // gorm.ErrRecordNotFound is expected if UUID is invalid
			s.log.Error().Str("service", "user").Err(err).Str("hashed_uuid", hashedUUID).Msg("Failed to find user by HashedUUID")
		}
		// Return a generic error or nil? Let's return nil, nil if not found, error otherwise.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // User not found
		}
		return nil, pkgErrors.Wrap(err, "failed to retrieve user details")
	}
	// Check expiry again just in case? Optional defense-in-depth.
	if user != nil && time.Now().After(user.DeletionDate) {
		s.log.Warn().Str("service", "user").Str("hashed_uuid", hashedUUID).Msg("User found by HashedUUID but is expired")
		return nil, ErrUserExpired // Return specific error if expired
	}

	return user, nil
}

// ResetAndRetrieveUserToken generates a new API token for the user, stores its hash,
// and returns the plain token.
func (s *service) ResetAndRetrieveUserToken(ctx context.Context, hashedUUID string) (string, error) {
	s.log.Debug().Str("service", "user").Str("hashed_uuid", hashedUUID).Msg("Attempting to reset and retrieve user token")

	// 1. Fetch the user by HashedUUID
	user, err := s.repo.FindByHashedUUID(ctx, hashedUUID)
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Str("hashed_uuid", hashedUUID).Msg("Failed to find user by HashedUUID for token reset")
		if errors.Is(err, gorm.ErrRecordNotFound) { // Check if gorm.ErrRecordNotFound
			return "", pkgErrors.Wrap(err, "user not found for token reset")
		}
		return "", pkgErrors.Wrap(err, "failed to retrieve user for token reset")
	}
	if user == nil { // Explicitly check for nil user if FindByHashedUUID can return nil, nil
		s.log.Warn().Str("service", "user").Str("hashed_uuid", hashedUUID).Msg("User not found by HashedUUID for token reset (nil user)")
		return "", pkgErrors.Wrap(gorm.ErrRecordNotFound, "user not found for token reset") // Return a gorm.ErrRecordNotFound like error
	}

	// 2. Generate new token and update user object in memory
	plainToken, err := s.generateAndStoreUserToken(user) // user object is updated here
	if err != nil {
		// Logging is handled within generateAndStoreUserToken
		return "", err // Return error from token generation/hashing
	}

	// 3. Persist the updated APITokenHash and Scopes to the database
	err = s.repo.UpdateTokenAndScopes(ctx, user.HashedUUID, user.APITokenHash, user.Scopes)
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Str("hashed_uuid", user.HashedUUID).Msg("Failed to update user token and scopes in repository")
		return "", pkgErrors.Wrap(err, "failed to store updated token and scopes")
	}

	s.log.Info().Str("service", "user").Str("hashed_uuid", user.HashedUUID).Msg("Successfully reset and stored new user token")
	// 4. Return the plain token
	return plainToken, nil
}

// handleAuthFailure increments the failure count and sets lockout if threshold is reached.
// Uses the provided key (e.g., token prefix or HashedUUID) for rate limiting.
func (s *service) handleAuthFailure(ctx context.Context, key string) {
	newCount, err := s.limiter.IncrementFailure(ctx, key)
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Str("key", key).Msg("Failed to increment rate limiter failure count")
		return // Don't proceed to lockout if increment failed
	}
	s.log.Warn().Str("service", "user").Str("key", key).Int64("failure_count", newCount).Msg("Authentication failure recorded")

	if newCount >= lockoutThreshold {
		err = s.limiter.SetLockout(ctx, key)
		if err != nil {
			s.log.Error().Str("service", "user").Err(err).Str("key", key).Msg("Failed to set rate limiter lockout")
		} else {
			s.log.Warn().Str("service", "user").Str("key", key).Msg("Rate limiter lockout threshold reached and lockout set")
		}
	}
}

const (
	profileUUIDTTL = 24 * time.Hour
	// When a UUID is promoted to the persistent database, we can extend its TTL in Valkey
	// to reduce the need for database lookups
	promotedProfileUUIDTTL = 7 * 24 * time.Hour // 7 days
)

// GetOrGenerateProfileUUID retrieves an existing profile-specific UUID from Valkey or the persistent database
// for a given sessionID, or generates, stores, and returns a new one if not found or expired.
func (s *service) GetOrGenerateProfileUUID(ctx context.Context, sessionID string) (string, error) {
	if sessionID == "" {
		s.log.Warn().Str("service", "user").Msg("SessionID is empty, cannot generate profile UUID")
		return "", pkgErrors.New("sessionID cannot be empty for profile UUID generation")
	}

	valkeyKey := fmt.Sprintf("session:%s:profile_uuid", sessionID)
	s.log.Debug().Str("service", "user").Str("valkey_key", valkeyKey).Msg("Attempting to get profile UUID from Valkey")

	client := s.valkeyService.GetClient()

	// Try to get the existing UUID
	existingUUID, err := client.Do(ctx, client.B().Get().Key(valkeyKey).Build()).ToString()
	// Check for actual errors, not just key-not-found
	if err != nil && !errors.Is(err, valkeyClient.Nil) { // Try errors.Is(err, valkeyClient.Nil)
		s.log.Error().Str("service", "user").Err(err).Str("valkey_key", valkeyKey).Msg("Failed to get profile UUID from Valkey")
		return "", pkgErrors.Wrap(err, "failed to retrieve profile UUID from Valkey")
	}

	// If UUID found: err would be nil.
	// If UUID not found: err would be valkeyClient.Nil.
	// If !errors.Is(err, valkeyClient.Nil) it means err is nil (found) or some other error (already handled).
	// So if err is nil (key found) AND existingUUID is not empty:
	if err == nil && existingUUID != "" {
		s.log.Info().Str("service", "user").Str("valkey_key", valkeyKey).Str("profile_uuid", existingUUID).Msg("Found existing profile UUID in Valkey, refreshing TTL")
		// Refresh TTL
		err = client.Do(ctx, client.B().Expire().Key(valkeyKey).Seconds(int64(profileUUIDTTL.Seconds())).Build()).Error()
		if err != nil {
			s.log.Error().Str("service", "user").Err(err).Str("valkey_key", valkeyKey).Msg("Failed to refresh TTL for profile UUID")
			// Continue with the existing UUID even if TTL refresh fails
		}
		return existingUUID, nil
	}

	// UUID not found in Valkey, check if it exists in the persistent database
	// Extract user ID from session ID (in this case, we're using HashedUUID as the user ID)
	userID := sessionID
	if userID != "" {
		persistentUUID, err := s.profileUUIDRepo.FindByUserID(ctx, userID)
		if err != nil {
			s.log.Error().Str("service", "user").Err(err).Str("user_id", userID).Msg("Failed to check persistent database for profile UUID")
			// Continue to generate a new UUID
		} else if persistentUUID != nil {
			s.log.Info().Str("service", "user").Str("user_id", userID).Str("profile_uuid", persistentUUID.ProfileUUID).Msg("Found existing profile UUID in persistent database, caching in Valkey")

			// Cache the UUID in Valkey with an extended TTL since it's a promoted UUID
			setCmd := client.B().Set().Key(valkeyKey).Value(persistentUUID.ProfileUUID).Ex(promotedProfileUUIDTTL).Build()
			if err := client.Do(ctx, setCmd).Error(); err != nil {
				s.log.Error().Str("service", "user").Err(err).Str("valkey_key", valkeyKey).Msg("Failed to cache persistent UUID in Valkey")
				// Continue with the UUID from the persistent database even if caching fails
			}

			// Update last activity timestamp
			if err := s.profileUUIDRepo.UpdateLastActivity(ctx, userID, persistentUUID.ProfileUUID); err != nil {
				s.log.Error().Str("service", "user").Err(err).Str("user_id", userID).Str("profile_uuid", persistentUUID.ProfileUUID).Msg("Failed to update last activity timestamp")
				// Continue with the UUID even if update fails
			}

			return persistentUUID.ProfileUUID, nil
		}
	}

	// UUID not found or expired, generate a new one
	s.log.Info().Str("service", "user").Str("valkey_key", valkeyKey).Msg("Profile UUID not found in Valkey or expired, generating a new one")
	newProfileUUID, uuidErr := uuid.NewRandom()
	if uuidErr != nil {
		s.log.Error().Str("service", "user").Err(uuidErr).Msg("Failed to generate new profile UUID")
		return "", pkgErrors.Wrap(uuidErr, "failed to generate new profile UUID")
	}
	newUUIDStr := newProfileUUID.String()

	// Store the new UUID in Valkey with TTL, using SET with NX and EX for atomicity
	// SET key value NX EX seconds
	// The valkey-go builder (from rueidis) typically chains these:
	// Set().Key().Value().Nx().Ex(duration).Build()
	setCmd := client.B().Set().Key(valkeyKey).Value(newUUIDStr).Nx().Ex(profileUUIDTTL).Build()
	resp := client.Do(ctx, setCmd)
	if err := resp.Error(); err != nil {
		// If err is valkeyClient.Nil, it means NX failed (key already existed). This is a race condition.
		// We should then re-fetch the value.
		if errors.Is(err, valkeyClient.Nil) { // Try errors.Is(err, valkeyClient.Nil)
			s.log.Info().Str("service", "user").Str("valkey_key", valkeyKey).Msg("SET NX failed, key already exists (race condition). Re-fetching profile UUID.")
			// Recursive call to re-fetch. Be mindful of potential deep recursion if contention is very high,
			// though unlikely for session-scoped UUIDs.
			return s.GetOrGenerateProfileUUID(ctx, sessionID)
		}
		s.log.Error().Str("service", "user").Err(err).Str("valkey_key", valkeyKey).Msg("Failed to store new profile UUID in Valkey using SET NX EX")
		return "", pkgErrors.Wrap(err, "failed to store new profile UUID in Valkey")
	}

	// Check if the SET NX operation was successful (i.e., the key was actually set)
	// For SET NX, a nil error and a non-"OK" response (like ValkeyNil if key already existed and NX prevented set)
	// or a specific response string might indicate it wasn't set.
	// However, valkey-go's Do().ToString() or similar might be needed to check the actual reply for "OK".
	// If SET NX fails because key already exists (race condition where another instance set it),
	// we should ideally re-fetch. For simplicity now, we assume if no error, it was set or already existed.
	// A more robust check:
	// resultStr, _ := resp.ToString()
	// if resultStr != "OK" {
	//   // This means another process set it first. We should re-fetch.
	//   s.log.Info().Str("service", "user").Str("valkey_key", valkeyKey).Msg("Profile UUID was set by another process, re-fetching")
	//   return s.GetOrGenerateProfileUUID(ctx, sessionID) // Recursive call, be careful with depth
	// }

	s.log.Info().Str("service", "user").Str("valkey_key", valkeyKey).Str("profile_uuid", newUUIDStr).Msg("Successfully generated and stored new profile UUID in Valkey")
	return newUUIDStr, nil
}

// PromoteProfileUUID promotes a profile UUID from Valkey to the persistent database.
// This is called during "data sync" events to ensure long-term persistence of actively used UUIDs.
func (s *service) PromoteProfileUUID(ctx context.Context, userID string, profileUUID string) error {
	if userID == "" || profileUUID == "" {
		return pkgErrors.New("userID and profileUUID cannot be empty for promotion")
	}

	// Check if the UUID already exists in the persistent database
	existingUUID, err := s.profileUUIDRepo.FindByUserID(ctx, userID)
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Str("user_id", userID).Msg("Failed to check if profile UUID exists in persistent database")
		return pkgErrors.Wrap(err, "failed to check if profile UUID exists in persistent database")
	}

	// If the UUID already exists and matches, just update the last activity timestamp
	if existingUUID != nil {
		if existingUUID.ProfileUUID == profileUUID {
			s.log.Info().Str("service", "user").Str("user_id", userID).Str("profile_uuid", profileUUID).Msg("Profile UUID already exists in persistent database, updating last activity")

			if err := s.profileUUIDRepo.UpdateLastActivity(ctx, userID, profileUUID); err != nil {
				s.log.Error().Str("service", "user").Err(err).Str("user_id", userID).Str("profile_uuid", profileUUID).Msg("Failed to update last activity timestamp")
				return pkgErrors.Wrap(err, "failed to update last activity timestamp")
			}

			// Extend TTL in Valkey since this is a promoted UUID
			client := s.valkeyService.GetClient()
			valkeyKey := fmt.Sprintf("session:%s:profile_uuid", userID)

			err = client.Do(ctx, client.B().Expire().Key(valkeyKey).Seconds(int64(promotedProfileUUIDTTL.Seconds())).Build()).Error()
			if err != nil {
				s.log.Error().Str("service", "user").Err(err).Str("valkey_key", valkeyKey).Msg("Failed to extend TTL for promoted profile UUID")
				// Continue even if TTL extension fails
			}

			return nil
		}

		// If the UUID exists but doesn't match, log a warning and update it
		s.log.Warn().Str("service", "user").Str("user_id", userID).
			Str("existing_uuid", existingUUID.ProfileUUID).
			Str("new_uuid", profileUUID).
			Msg("User has a different profile UUID in persistent database, updating to new UUID")
	}

	// Store the UUID in the persistent database
	now := time.Now()
	newProfileUUID := domain.ProfileUUID{
		UserID:         userID,
		ProfileUUID:    profileUUID,
		CreatedAt:      now,
		LastActivityAt: now,
	}

	if err := s.profileUUIDRepo.Store(ctx, newProfileUUID); err != nil {
		s.log.Error().Str("service", "user").Err(err).Str("user_id", userID).Str("profile_uuid", profileUUID).Msg("Failed to store profile UUID in persistent database")
		return pkgErrors.Wrap(err, "failed to store profile UUID in persistent database")
	}

	// Extend TTL in Valkey since this is a promoted UUID
	client := s.valkeyService.GetClient()
	valkeyKey := fmt.Sprintf("session:%s:profile_uuid", userID)

	err = client.Do(ctx, client.B().Expire().Key(valkeyKey).Seconds(int64(promotedProfileUUIDTTL.Seconds())).Build()).Error()
	if err != nil {
		s.log.Error().Str("service", "user").Err(err).Str("valkey_key", valkeyKey).Msg("Failed to extend TTL for promoted profile UUID")
		// Continue even if TTL extension fails
	}

	s.log.Info().Str("service", "user").Str("user_id", userID).Str("profile_uuid", profileUUID).Msg("Successfully promoted profile UUID to persistent database")
	return nil
}
