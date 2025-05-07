package database

import (
	"context"
	"fmt"
	"log" // Standard log package for GORM logger adapter
	"os"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/flurbudurbur/Shiori/pkg/errors"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type DB struct {
	log     zerolog.Logger
	handler *gorm.DB
	ctx     context.Context
	cancel  func()

	Driver string
	DSN    string
}

func NewDB(cfg *domain.Config, log logger.Logger) (*DB, error) {
	db := &DB{
		log: log.With().Str("module", "database").Logger(),
	}
	db.ctx, db.cancel = context.WithCancel(context.Background())

	switch cfg.Database.Type { // Access via nested Database struct
	case "sqlite":
		db.Driver = "sqlite"
		// Assuming dataSourceName is defined elsewhere (e.g., utils.go)
		db.DSN = dataSourceName(cfg.ConfigPath, "syncyomi.db")
	case "postgres", "postgresql":
		// Access via nested Database.Postgres struct
		if cfg.Database.Postgres.Host == "" || cfg.Database.Postgres.Port == 0 || cfg.Database.Postgres.Database == "" {
			return nil, errors.New("postgres configuration is incomplete")
		}
		// Construct DSN for PostgreSQL
		// Access via nested Database.Postgres struct
		db.DSN = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Database.Postgres.Host, cfg.Database.Postgres.Port, cfg.Database.Postgres.User, cfg.Database.Postgres.Pass, cfg.Database.Postgres.Database, cfg.Database.Postgres.SslMode)
		db.Driver = "postgres"
	default:
		return nil, errors.New("unsupported database type: %v", cfg.Database.Type) // Access via nested Database struct
	}

	return db, nil
}

func (db *DB) Open() error {
	if db.DSN == "" {
		return errors.New("database DSN is required but not configured")
	}

	var dialector gorm.Dialector
	// Configure GORM logger
	gormLogLevel := gormlogger.Warn // Default level
	switch db.log.GetLevel() {
	case zerolog.InfoLevel:
		gormLogLevel = gormlogger.Info
	case zerolog.WarnLevel:
		gormLogLevel = gormlogger.Warn
	case zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel:
		gormLogLevel = gormlogger.Error
	case zerolog.DebugLevel, zerolog.TraceLevel:
		// GORM's Info level is closest to Debug/Trace
		gormLogLevel = gormlogger.Info
	default:
		// Use Silent for levels lower than Debug (e.g., Disabled)
		gormLogLevel = gormlogger.Silent
	}

	// Using standard logger adapter for simplicity.
	// For more advanced integration with zerolog, a custom GORM logger implementation would be needed.
	newLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond, // Slow SQL threshold
			LogLevel:                  gormLogLevel,           // Log level
			IgnoreRecordNotFoundError: true,                   // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,                  // Disable color
		},
	)

	gormConfig := &gorm.Config{
		Logger: newLogger,
	}

	switch db.Driver {
	case "sqlite":
		dialector = sqlite.Open(db.DSN)
		db.log.Info().Str("dsn", db.DSN).Msg("Using SQLite driver")
	case "postgres":
		dialector = postgres.Open(db.DSN)
		db.log.Info().Msg("Using PostgreSQL driver") // Avoid logging DSN with password
	default:
		return errors.New("unsupported database driver: %s", db.Driver)
	}

	gormDB, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		db.log.Error().Err(err).Str("driver", db.Driver).Msg("Failed to connect database")
		return errors.Wrap(err, "failed to connect database")
	}
	db.handler = gormDB
	db.log.Info().Msg("Database connection established successfully.")

	// Run Migrations
	db.log.Info().Msg("Running database auto-migrations...")
	err = db.handler.AutoMigrate(
		&domain.Notification{},
		&domain.User{},
		&SyncData{},           // Add the new SyncData model for migration (Removed leading '+')
		&domain.ProfileUUID{}, // Add the ProfileUUID model for migration
		// Add any other domain models that need tables here in the future
	)
	if err != nil {
		db.log.Error().Err(err).Msg("Failed to run database auto-migrations")
		return errors.Wrap(err, "failed to run database auto-migrations")
	}
	db.log.Info().Msg("Database auto-migrations completed.")

	return nil
}

func (db *DB) Close() error {
	// Cancel background context
	db.cancel()

	// GORM manages the underlying connection pool.
	// Explicitly closing the *sql.DB instance might be necessary in some specific scenarios,
	// but generally, it's handled by GORM. If needed:
	// sqlDB, err := db.handler.DB()
	// if err == nil && sqlDB != nil {
	// 	 db.log.Info().Msg("Closing underlying database connection.")
	//	 return sqlDB.Close()
	// } else if err != nil {
	//   db.log.Error().Err(err).Msg("Failed to get underlying DB for closing.")
	//   return err
	// }
	db.log.Info().Msg("Database service closed.")
	return nil
}

func (db *DB) Ping() error {
	if db.handler == nil {
		return errors.New("database handler is not initialized")
	}
	sqlDB, err := db.handler.DB()
	if err != nil {
		db.log.Error().Err(err).Msg("Failed to get underlying *sql.DB for ping")
		return errors.Wrap(err, "failed to get underlying *sql.DB")
	}

	err = sqlDB.PingContext(db.ctx)
	if err != nil {
		db.log.Warn().Err(err).Msg("Database ping failed")
		return errors.Wrap(err, "database ping failed")
	}
	db.log.Debug().Msg("Database ping successful")
	return nil
}

// Get returns the underlying GORM DB instance.
// This allows repository implementations to use the GORM handler directly.
func (db *DB) Get() *gorm.DB {
	return db.handler
}
