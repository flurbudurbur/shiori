package auth

import (
	"context"

	"github.com/flurbudurbur/Shiori/internal/domain" // Add domain import
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/internal/user"

	// "github.com/google/uuid" // No longer needed here as userSvc handles UUID generation
	"github.com/rs/zerolog"
)

// Service interface now matches http.authService
type Service interface {
	GetUserCount(ctx context.Context) (int, error)
	GenerateUserBookmark(ctx context.Context) (hashedUUID string, err error)
	AuthenticateUser(ctx context.Context, hashedUUID string) (*domain.User, error)
}

type service struct {
	log     zerolog.Logger
	userSvc user.Service
}

func NewService(log logger.Logger, userSvc user.Service) Service {
	return &service{
		log:     log.With().Str("module", "auth").Logger(),
		userSvc: userSvc,
	}
}

func (s *service) GetUserCount(ctx context.Context) (int, error) {
	return s.userSvc.GetUserCount(ctx)
}

// GenerateUserBookmark now calls the user service's RegisterNewUser,
// which creates the user and returns the HashedUUID.
func (s *service) GenerateUserBookmark(ctx context.Context) (string, error) {
	hashedUUID, err := s.userSvc.RegisterNewUser(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to generate user bookmark via user service")
		return "", err
	}
	s.log.Info().Str("hashed_uuid", hashedUUID).Msg("Successfully generated user bookmark")
	return hashedUUID, nil
}

// AuthenticateUser delegates to userSvc.GetUserForAuthentication,
// as we are authenticating with the HashedUUID.
func (s *service) AuthenticateUser(ctx context.Context, hashedUUID string) (*domain.User, error) { // Renamed return variable to avoid conflict
	// The providedUUID is the HashedUUID.
	// AuthenticateUserByToken is for API tokens, not direct UUID login.
	// We should use GetUserForAuthentication.
	foundUser, err := s.userSvc.GetUserForAuthentication(ctx, hashedUUID) // Use new variable name
	if err != nil {
		s.log.Warn().Err(err).Str("hashed_uuid_prefix", hashedUUID[:min(len(hashedUUID), 8)]).Msg("Authentication failed for UUID")
		return nil, err // Propagate error (includes gorm.ErrRecordNotFound)
	}
	if foundUser == nil { // Check the new variable name
		s.log.Warn().Str("hashed_uuid_prefix", hashedUUID[:min(len(hashedUUID), 8)]).Msg("User not found for UUID authentication")
		// Now 'user.ErrAuthenticationFailed' correctly refers to the package 'user'
		return nil, user.ErrAuthenticationFailed
	}
	// Potentially check user.DeletionDate here if not handled by GetUserForAuthentication
	return foundUser, nil // Return the new variable name
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RegisterNewUser method is removed from this service as GenerateUserBookmark covers its role.
// The http.authService interface also doesn't require it anymore.
