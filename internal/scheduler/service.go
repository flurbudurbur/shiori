package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/internal/notification"
	"github.com/flurbudurbur/Shiori/internal/update"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
)

type Service interface {
	Start()
	Stop()
	// AddJob adds a job that runs periodically at the given interval.
	AddJob(job cron.Job, interval time.Duration, identifier string) (int, error)
	// AddJobWithSpec adds a job using a cron spec string (e.g., "0 3 * * *").
	AddJobWithSpec(job cron.Job, spec string, identifier string) (int, error)
	RemoveJobByIdentifier(id string) error
	GetNextRun(id string) (time.Time, error)
}

type service struct {
	log             zerolog.Logger
	config          *domain.Config
	version         string
	notificationSvc notification.Service
	updateSvc       *update.Service
	userRepo        domain.UserRepo
	profileUUIDRepo domain.ProfileUUIDRepo // Add ProfileUUIDRepo

	cron *cron.Cron
	jobs map[string]cron.EntryID
	m    sync.RWMutex
}

// Update NewService to accept ProfileUUIDRepo
func NewService(log logger.Logger, config *domain.Config, notificationSvc notification.Service,
	updateSvc *update.Service, userRepo domain.UserRepo, profileUUIDRepo domain.ProfileUUIDRepo) Service {
	return &service{
		log:             log.With().Str("module", "scheduler").Logger(),
		config:          config,
		notificationSvc: notificationSvc,
		updateSvc:       updateSvc,
		userRepo:        userRepo,
		profileUUIDRepo: profileUUIDRepo, // Store ProfileUUIDRepo
		cron: cron.New(cron.WithChain(
			cron.Recover(cron.DefaultLogger), // Add recovery middleware
		)),
		jobs: map[string]cron.EntryID{},
	}
}

func (s *service) Start() {
	s.log.Info().Msg("Starting scheduler service") // Use Info level for start/stop

	// start scheduler
	s.cron.Start()

	// init jobs
	go s.addAppJobs() // Run in goroutine to not block startup

	return
}

func (s *service) addAppJobs() {
	// Small delay to ensure other services might be ready
	time.Sleep(5 * time.Second)
	s.log.Info().Msg("Adding application-specific scheduled jobs")

	// --- Add CheckUpdatesJob ---
	if s.config.CheckForUpdates {
		checkUpdates := &CheckUpdatesJob{
			Name:             "app-check-updates",
			Log:              s.log.With().Str("job", "app-check-updates").Logger(),
			Version:          s.version, // Need to ensure version is populated correctly
			NotifSvc:         s.notificationSvc,
			updateService:    s.updateSvc,
			lastCheckVersion: s.version,
		}

		// Schedule to run every 2 hours
		if _, err := s.AddJob(checkUpdates, 2*time.Hour, "app-check-updates"); err != nil {
			s.log.Error().Err(err).Msg("Failed to add 'app-check-updates' job")
		}
	} else {
		s.log.Info().Msg("Update checking is disabled, skipping 'app-check-updates' job")
	}

	// --- Add PruneExpiredUsersJob ---
	pruneUsersJob := &PruneExpiredUsersJob{
		Name: "app-prune-expired-users",
		Log:  s.log.With().Str("job", "app-prune-expired-users").Logger(),
		Repo: s.userRepo,
	}

	// Schedule to run daily at 3 AM server time (as per plan)
	// Cron spec: "minute hour day-of-month month day-of-week"
	// "0 3 * * *" means at minute 0 of hour 3 on every day, month, and day of week.
	pruneSpec := "0 3 * * *"
	if _, err := s.AddJobWithSpec(pruneUsersJob, pruneSpec, "app-prune-expired-users"); err != nil {
		s.log.Error().Err(err).Msg("Failed to add 'app-prune-expired-users' job")
	}

	// --- Add ProfileUUIDCleanupJob ---
	profileUUIDCleanupJob := &ProfileUUIDCleanupJob{
		Name:   "app-profile-uuid-cleanup",
		Log:    s.log.With().Str("job", "app-profile-uuid-cleanup").Logger(),
		Repo:   s.profileUUIDRepo,
		Config: &s.config.UUIDCleanup,
	}

	// Use the schedule from configuration
	cleanupSpec := s.config.UUIDCleanup.Schedule
	if cleanupSpec == "" {
		cleanupSpec = "0 3 * * *"
	}

	if _, err := s.AddJobWithSpec(profileUUIDCleanupJob, cleanupSpec, "app-profile-uuid-cleanup"); err != nil {
		s.log.Error().Err(err).Msg("Failed to add 'app-profile-uuid-cleanup' job")
	} else {
		s.log.Info().
			Str("schedule", cleanupSpec).
			Int("inactivity_days", s.config.UUIDCleanup.InactivityDays).
			Bool("delete_orphaned", s.config.UUIDCleanup.DeleteOrphanedUUIDs).
			Bool("use_soft_delete", s.config.UUIDCleanup.UseSoftDelete).
			Msg("Profile UUID cleanup job scheduled")
	}

	s.log.Info().Msg("Finished adding application-specific scheduled jobs")
}

func (s *service) Stop() {
	s.log.Info().Msg("Stopping scheduler service") // Use Info level
	s.cron.Stop()
	return
}

func (s *service) AddJob(job cron.Job, interval time.Duration, identifier string) (int, error) {
	s.m.Lock()
	defer s.m.Unlock()

	if _, exists := s.jobs[identifier]; exists {
		s.log.Warn().Str("identifier", identifier).Msg("Job with this identifier already exists, skipping add.")
		// Return existing ID? Or error? Let's return error for now.
		return 0, fmt.Errorf("job with identifier '%s' already exists", identifier)
	}

	entryID, err := s.cron.AddJob(fmt.Sprintf("@every %s", interval.String()), cron.NewChain(
		cron.SkipIfStillRunning(cron.DefaultLogger)).Then(job)) // Use DefaultLogger for skip logs

	if err != nil {
		s.log.Error().Err(err).Str("identifier", identifier).Msg("Failed to add job with interval")
		return 0, fmt.Errorf("failed to add job '%s': %w", identifier, err)
	}

	s.log.Info().Str("identifier", identifier).Dur("interval", interval).Int("entryID", int(entryID)).Msg("Scheduled job added")
	s.jobs[identifier] = entryID
	return int(entryID), nil
}

// AddJobWithSpec adds a job using a cron specification string.
func (s *service) AddJobWithSpec(job cron.Job, spec string, identifier string) (int, error) {
	s.m.Lock()
	defer s.m.Unlock()

	if _, exists := s.jobs[identifier]; exists {
		s.log.Warn().Str("identifier", identifier).Msg("Job with this identifier already exists, skipping add.")
		return 0, fmt.Errorf("job with identifier '%s' already exists", identifier)
	}

	entryID, err := s.cron.AddJob(spec, cron.NewChain(
		cron.SkipIfStillRunning(cron.DefaultLogger)).Then(job)) // Use DefaultLogger for skip logs

	if err != nil {
		s.log.Error().Err(err).Str("identifier", identifier).Str("spec", spec).Msg("Failed to add job with spec")
		return 0, fmt.Errorf("failed to add job '%s' with spec '%s': %w", identifier, spec, err)
	}

	s.log.Info().Str("identifier", identifier).Str("spec", spec).Int("entryID", int(entryID)).Msg("Scheduled job added")
	s.jobs[identifier] = entryID
	return int(entryID), nil
}

func (s *service) RemoveJobByIdentifier(id string) error {
	s.m.Lock()
	defer s.m.Unlock()

	v, ok := s.jobs[id]
	if !ok {
		return nil
	}

	s.log.Debug().Msgf("scheduler.Remove: removing job: %v", id)

	// remove from cron
	s.cron.Remove(v)

	// remove from jobs map
	delete(s.jobs, id)

	return nil
}

func (s *service) GetNextRun(id string) (time.Time, error) {
	entry := s.getEntryById(id)

	if !entry.Valid() {
		return time.Time{}, nil
	}

	s.log.Debug().Msgf("scheduler.GetNextRun: %s next run: %s", id, entry.Next)

	return entry.Next, nil
}

func (s *service) getEntryById(id string) cron.Entry {
	s.m.Lock()
	defer s.m.Unlock()

	v, ok := s.jobs[id]
	if !ok {
		return cron.Entry{}
	}

	return s.cron.Entry(v)
}

type GenericJob struct {
	Name string
	Log  zerolog.Logger

	callback func()
}

func (j *GenericJob) Run() {
	j.callback()
}
