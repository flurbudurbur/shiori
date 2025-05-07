package valkey

import (
	"context"
	"fmt"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/valkey-io/valkey-go"
)

// Service holds the Valkey client and provides methods to interact with Valkey.
type Service struct {
	client valkey.Client
	config domain.ValkeyConfig
}

// NewService creates and returns a new Valkey service.
// It initializes the Valkey client based on the provided configuration.
func NewService(cfg domain.ValkeyConfig) (*Service, error) {
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{cfg.Address},
		Password:    cfg.Password,
		SelectDB:    cfg.DB,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	// Ping the server to ensure connection is established.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping Valkey: %w", err)
	}

	return &Service{
		client: client,
		config: cfg,
	}, nil
}

// Close closes the Valkey client connection.
func (s *Service) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// GetClient returns the underlying Valkey client.
// This can be used if direct access to the client is needed for operations not exposed by the service.
func (s *Service) GetClient() valkey.Client {
	return s.client
}
