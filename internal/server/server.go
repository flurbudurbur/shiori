package server

import (
	"context"
	"sync"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/internal/scheduler"
	"github.com/flurbudurbur/Shiori/internal/update"
	"github.com/rs/zerolog"
)

type Server struct {
	log    zerolog.Logger
	config *domain.Config

	scheduler     scheduler.Service
	updateService *update.Service

	stopWG sync.WaitGroup
	lock   sync.Mutex
}

func NewServer(log logger.Logger, config *domain.Config, scheduler scheduler.Service, updateSvc *update.Service) *Server {
	return &Server{
		log:           log.With().Str("module", "server").Logger(),
		config:        config,
		scheduler:     scheduler,
		updateService: updateSvc,
	}
}

func (s *Server) Start() error {
	go s.checkUpdates()

	// start cron scheduler
	s.scheduler.Start()

	return nil
}

func (s *Server) Shutdown() {
	s.log.Info().Msg("Shutting down server")

	// stop cron scheduler
	s.scheduler.Stop()
}

func (s *Server) checkUpdates() {
	if s.config.CheckForUpdates {
		time.Sleep(1 * time.Second)

		s.updateService.CheckUpdates(context.Background())
	}
}
