package http

import (
	"net/http"

	"github.com/flurbudurbur/Shiori/internal/domain"
	userservice "github.com/flurbudurbur/Shiori/internal/user" // Renamed import for clarity
	"github.com/rs/zerolog"
)

// UserResource holds dependencies for user-related HTTP handlers.
type UserResource struct {
	userService userservice.Service // Use userservice.Service from internal/user package
	log         zerolog.Logger      // Use zerolog.Logger directly
	encoder     encoder             // Assuming 'encoder' is a defined type/interface for JSON responses
}

// NewUserResource creates a new UserResource.
func NewUserResource(userService userservice.Service, log zerolog.Logger, enc encoder) *UserResource {
	return &UserResource{
		userService: userService,
		log:         log.With().Str("resource", "user").Logger(), // Specialize logger for this resource
		encoder:     enc,
	}
}

// handleResetAndGetUserToken handles the request to reset and retrieve a user's API token.
func (ur *UserResource) handleResetAndGetUserToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Retrieve authenticated user from context
	user, ok := ctx.Value("user").(*domain.User) // Assuming "user" is the key used by auth middleware
	if !ok || user == nil {
		ur.log.Warn().Msg("User not found in context or type assertion failed for token reset")
		ur.encoder.StatusResponse(ctx, w, map[string]string{"error": "Unauthorized: User context not available"}, http.StatusUnauthorized)
		return
	}

	ur.log.Debug().Str("hashed_uuid", user.HashedUUID).Msg("Attempting to reset and retrieve token for user")

	// 2. Call the service method
	plainToken, err := ur.userService.ResetAndRetrieveUserToken(ctx, user.HashedUUID)
	if err != nil {
		ur.log.Error().Err(err).Str("hashed_uuid", user.HashedUUID).Msg("Failed to reset and retrieve user token via service")
		// Consider checking for specific error types from the service if needed (e.g., user not found)
		ur.encoder.StatusResponse(ctx, w, map[string]string{"error": "Failed to reset API token"}, http.StatusInternalServerError)
		return
	}

	// 3. Return the plain token in the response
	response := map[string]string{"api_token": plainToken}
	ur.log.Info().Str("hashed_uuid", user.HashedUUID).Msg("Successfully reset and provided API token to user")
	ur.encoder.StatusResponse(ctx, w, response, http.StatusOK)
}

// handleGetProfile is a placeholder for fetching profile data.
// It will generate/retrieve a profile-specific UUID using Valkey.
func (ur *UserResource) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Retrieve authenticated user from context (primarily to ensure user is authenticated)
	user, ok := ctx.Value("user").(*domain.User) // Assuming "user" is the key used by auth middleware
	if !ok || user == nil {
		ur.log.Warn().Msg("User not found in context for profile access")
		ur.encoder.StatusResponse(ctx, w, map[string]string{"error": "Unauthorized: User context not available"}, http.StatusUnauthorized)
		return
	}

	// 2. Obtain a session identifier.
	//    Attempt to get a specific sessionID from context.
	//    If not available, this part needs clarification or use user.HashedUUID as a fallback (with a note).
	var sessionID string
	sessionIDVal := ctx.Value("sessionID") // Assuming "sessionID" is a key used by session middleware
	if sessionIDVal != nil {
		sessionID, ok = sessionIDVal.(string)
		if !ok || sessionID == "" {
			ur.log.Warn().Str("hashed_uuid", user.HashedUUID).Msg("SessionID found in context but is not a non-empty string. Falling back to HashedUUID for profile UUID key.")
			// Fallback or error? For now, let's try to proceed with HashedUUID as a session-like identifier for this specific UUID.
			// This is an assumption if a true ephemeral session ID isn't readily available.
			sessionID = user.HashedUUID // Using HashedUUID as a stand-in if specific sessionID is not found/valid.
		}
	} else {
		ur.log.Warn().Str("hashed_uuid", user.HashedUUID).Msg("SessionID not found in context. Falling back to HashedUUID for profile UUID key.")
		sessionID = user.HashedUUID // Fallback
	}

	if sessionID == "" { // Should not happen if user is present and HashedUUID is mandatory
		ur.log.Error().Str("hashed_uuid", user.HashedUUID).Msg("Critical: sessionID became empty even after fallback for profile UUID generation.")
		ur.encoder.StatusResponse(ctx, w, map[string]string{"error": "Internal server error: could not determine session identifier"}, http.StatusInternalServerError)
		return
	}

	ur.log.Debug().Str("hashed_uuid", user.HashedUUID).Str("session_id_for_key", sessionID).Msg("Attempting to get/generate profile UUID")

	// 3. Call the user service to get/generate the profile UUID
	profileUUID, err := ur.userService.GetOrGenerateProfileUUID(ctx, sessionID)
	if err != nil {
		ur.log.Error().Err(err).Str("session_id_for_key", sessionID).Msg("Failed to get or generate profile UUID")
		ur.encoder.StatusResponse(ctx, w, map[string]string{"error": "Failed to process profile request"}, http.StatusInternalServerError)
		return
	}

	ur.log.Info().Str("hashed_uuid", user.HashedUUID).Str("session_id_for_key", sessionID).Str("profile_uuid", profileUUID).Msg("Successfully retrieved/generated profile UUID")

	// Promote the profile UUID to the persistent database when the user accesses their profile
	// This ensures that actively used profiles have their UUIDs persisted
	if promoteErr := ur.userService.PromoteProfileUUID(ctx, user.HashedUUID, profileUUID); promoteErr != nil {
		// Log the error but don't fail the profile access operation
		ur.log.Error().Err(promoteErr).Str("hashed_uuid", user.HashedUUID).Str("profile_uuid", profileUUID).Msg("Failed to promote profile UUID to persistent database")
		// Continue with the profile access even if promotion fails
	} else {
		ur.log.Info().Str("hashed_uuid", user.HashedUUID).Str("profile_uuid", profileUUID).Msg("Successfully promoted profile UUID to persistent database")
	}

	// For this subtask, we don't need to do anything further with the profileUUID in the response.
	// This handler would typically proceed to fetch actual profile data.
	response := map[string]interface{}{
		"message":      "Profile access successful (UUID managed).",
		"profile_uuid": profileUUID, // Included for demonstration/logging, not necessarily for direct frontend use yet
		"user_id":      user.HashedUUID,
	}
	ur.encoder.StatusResponse(ctx, w, response, http.StatusOK)
}
