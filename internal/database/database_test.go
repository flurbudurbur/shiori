package database

import (
	"os"
	"path" // Changed from path/filepath
	"testing"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// "strings" // Removed unused import
)

func newTestConfig(dbType string, configPath string) *domain.Config {
	cfg := &domain.Config{
		ConfigPath: configPath,
		Database: domain.DatabaseConfig{
			Type: dbType,
			Postgres: domain.PostgresConfig{ // Provide defaults even if not used by current test
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Pass:     "pass",
				Database: "testdb",
				SslMode:  "disable",
			},
		},
		Logging: domain.LoggingConfig{Level: "DEBUG"}, // For GORM logger
	}
	if dbType == "sqlite" && configPath == "" {
		// For in-memory or specific test file, dataSourceName might need adjustment
		// but NewDB handles default naming if configPath is provided.
	}
	return cfg
}

func TestNewDB_SQLite(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestConfig("sqlite", tmpDir)
	log := logger.Mock()

	db, err := NewDB(cfg, log)
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.Equal(t, "sqlite", db.Driver)
	expectedDSN := path.Join(tmpDir, "syncyomi.db") // Changed filepath.Join to path.Join
	assert.Equal(t, expectedDSN, db.DSN)
}

func TestNewDB_Postgres(t *testing.T) {
	cfg := newTestConfig("postgres", "")
	cfg.Database.Postgres = domain.PostgresConfig{
		Host:     "pg_host",
		Port:     5433,
		User:     "pg_user",
		Pass:     "pg_pass",
		Database: "pg_db",
		SslMode:  "require",
	}
	log := logger.Mock()

	db, err := NewDB(cfg, log)
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.Equal(t, "postgres", db.Driver)
	expectedDSN := "host=pg_host port=5433 user=pg_user password=pg_pass dbname=pg_db sslmode=require"
	assert.Equal(t, expectedDSN, db.DSN)
}

func TestNewDB_Postgres_IncompleteConfig(t *testing.T) {
	cfg := newTestConfig("postgres", "")
	cfg.Database.Postgres.Host = "" // Missing host
	log := logger.Mock()

	_, err := NewDB(cfg, log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postgres configuration is incomplete")
}

func TestNewDB_UnsupportedType(t *testing.T) {
	cfg := newTestConfig("mysql", "")
	log := logger.Mock()

	_, err := NewDB(cfg, log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database type: mysql")
}

func setupTestDBInstance(t *testing.T, useInMemoryAndShared bool) (*DB, func()) { // Renamed and clarified bool
	t.Helper()
	log := logger.Mock()
	var cfg *domain.Config
	var dbPath string // This will store the DSN or the file path for cleanup
	var actualDSNForNewDB string

	if useInMemoryAndShared {
		// Use a shared in-memory database. This is good for speed and isolation.
		// Migrations will run, but data won't persist if the *DB instance is lost.
		actualDSNForNewDB = "file::memory:?cache=shared"
		dbPath = actualDSNForNewDB // For cleanup logic, though os.Remove won't apply
		cfg = newTestConfig("sqlite", "") // ConfigPath is not strictly needed for in-memory DSN
	} else {
		// Use a temporary file-based SQLite database.
		// This is better for testing persistence across re-opens if needed,
		// and for ensuring migrations create actual file structures.
		tempDir := t.TempDir()
		cfg = newTestConfig("sqlite", tempDir) // NewDB will use this ConfigPath
		// NewDB will create "syncyomi.db" in tempDir.
		dbPath = path.Join(tempDir, "syncyomi.db") // Changed filepath.Join to path.Join
		actualDSNForNewDB = dbPath // NewDB will construct this DSN internally
	}

	dbInstance, err := NewDB(cfg, log)
	require.NoError(t, err)

	// If we used a specific DSN for in-memory, ensure the dbInstance uses it.
	// Otherwise, NewDB would have constructed it based on cfg.ConfigPath.
	if useInMemoryAndShared {
		dbInstance.DSN = actualDSNForNewDB
	}
	// For file-based, dbInstance.DSN is already correctly set by NewDB.

	err = dbInstance.Open()
	require.NoError(t, err)

	cleanup := func() {
		errClose := dbInstance.Close()
		assert.NoError(t, errClose, "Error closing test DB")
		// The check `!strings.Contains(dbPath, ":memory:")` is no longer needed
		// because `useInMemoryAndShared` directly tells us if it was an in-memory DSN.
		if !useInMemoryAndShared && dbPath != "" {
			errRemove := os.Remove(dbPath)
			if errRemove != nil && !os.IsNotExist(errRemove) {
				t.Logf("Warning: error removing test DB file %s: %v", dbPath, errRemove)
			}
		}
	}
	return dbInstance, cleanup
}
// If strings is no longer used after this change, the import will be removed in a later step if caught by linter.

func TestDB_Open_Close_Ping_Get(t *testing.T) {
	db, cleanup := setupTestDBInstance(t, false) // Use file-based temp SQLite
	defer cleanup()

	require.NotNil(t, db.handler, "DB handler should be initialized after Open")

	err := db.Ping()
	assert.NoError(t, err, "Ping should succeed on open DB")

	gormDB := db.Get()
	assert.NotNil(t, gormDB, "Get() should return a non-nil GORM DB")

	// Test Ping when handler is nil (before Open or after failed Open)
	dbNoHandler := &DB{log: logger.Mock().With().Str("module", "database").Logger()}
	err = dbNoHandler.Ping()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database handler is not initialized")
}

func TestDB_Open_Migrations(t *testing.T) {
	db, cleanup := setupTestDBInstance(t, false) // Use file-based temp SQLite
	defer cleanup()

	// Check if tables were created by AutoMigrate
	tables := []string{"notifications", "users", "sync_data", "profile_uuids"}
	for _, table := range tables {
		hasTable := db.handler.Migrator().HasTable(table)
		assert.True(t, hasTable, "Table %s should exist after migration", table)
	}
}

func TestDB_Open_NoDSN(t *testing.T) {
	log := logger.Mock()
	dbInstance := &DB{log: log.With().Str("module", "database").Logger(), Driver: "sqlite"} // DSN is empty
	err := dbInstance.Open()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database DSN is required")
}

func TestDB_Open_UnsupportedDriverInOpen(t *testing.T) {
	log := logger.Mock()
	dbInstance := &DB{
		log:    log.With().Str("module", "database").Logger(),
		Driver: "oracle", // Unsupported by our Open()
		DSN:    "some_dsn",
	}
	err := dbInstance.Open()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database driver: oracle")
}

func TestDB_Open_GormOpenError_SQLite_InvalidDSN(t *testing.T) {
	log := logger.Mock()
	// Create a temporary directory to ensure cfg.ConfigPath is valid for NewDB,
	// even though we'll override the DSN later.
	tmpDirForNewDB := t.TempDir()
	cfg := newTestConfig("sqlite", tmpDirForNewDB) // NewDB needs a valid ConfigPath for SQLite DSN generation

	dbInstance, err := NewDB(cfg, log)
	require.NoError(t, err)
	require.NotNil(t, dbInstance)

	// Override DSN to be a directory path, which SQLite driver should fail to open as a database file.
	// Create another temp dir to use as the invalid DSN.
	invalidDSNDir := t.TempDir()
	dbInstance.DSN = invalidDSNDir // Using a directory as a DSN

	err = dbInstance.Open()
	require.Error(t, err, "gorm.Open should fail when DSN is a directory for SQLite")
	// The specific error message from SQLite can vary, e.g., "unable to open database file", "is a directory"
	// GORM wraps this, so we check for our wrapper message.
	assert.Contains(t, err.Error(), "failed to connect database")
	t.Logf("Successfully tested gorm.Open failure with invalid DSN: %v", err)
}

// Note: Testing PostgreSQL connection and migrations would typically require a running
// PostgreSQL instance and appropriate configuration (e.g., via environment variables for CI).
// These tests focus on the SQLite path and the general logic of the DB struct.