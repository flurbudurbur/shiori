package database

import (
	"context"
	"database/sql"
	"log" // Standard log for GORM logger
	"os"  // For os.Stdout if needed by GORM logger
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	// "github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger" // For logger.Mock()
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres" // Using postgres driver for GORM, can be any dialect for mock
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger" // For gormlogger.Silent
)

// newMockDB creates a new GORM DB instance with a sqlmock.
func newMockDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()
	mockSqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)

	// Configure a silent GORM logger for tests
	silentLogger := gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer, can be io.Discard for truly silent
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Silent, // Set log level to Silent
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: mockSqlDB,
	}), &gorm.Config{
		Logger: silentLogger, // Use the configured silent logger
	})
	require.NoError(t, err)

	db := &DB{
		handler: gormDB,
		log:     logger.Mock().With().Logger(),
	}
	return db, mock
}

func TestNewSyncRepo(t *testing.T) {
	log := logger.Mock()
	db, _ := newMockDB(t)

	repo := NewSyncRepo(log, db)
	assert.NotNil(t, repo)

	syncRepo, ok := repo.(*SyncRepo)
	assert.True(t, ok, "NewSyncRepo should return a *SyncRepo type")
	assert.NotNil(t, syncRepo.db, "DB should be assigned in SyncRepo")
	assert.NotNil(t, syncRepo.log, "Logger should be assigned in SyncRepo")
}

func TestSyncRepo_GetSyncDataETag(t *testing.T) {
	ctx := context.Background()
	log := logger.Mock()
	apiKey := "test-api-key"
	expectedETag := "etag-123"

	t.Run("ETag found", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		rows := sqlmock.NewRows([]string{"data_etag"}).AddRow(expectedETag)
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT "data_etag" FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnRows(rows)

		etag, err := repo.GetSyncDataETag(ctx, apiKey)
		require.NoError(t, err)
		require.NotNil(t, etag)
		assert.Equal(t, expectedETag, *etag)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Record not found", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT "data_etag" FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnError(gorm.ErrRecordNotFound)

		etag, err := repo.GetSyncDataETag(ctx, apiKey)
		require.NoError(t, err)
		assert.Nil(t, etag)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Database error", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT "data_etag" FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnError(sql.ErrConnDone)

		etag, err := repo.GetSyncDataETag(ctx, apiKey)
		require.Error(t, err)
		assert.Nil(t, etag)
		assert.Contains(t, err.Error(), "failed to get sync data ETag")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSyncRepo_GetSyncDataAndETag(t *testing.T) {
	ctx := context.Background()
	log := logger.Mock()
	apiKey := "test-api-key"
	expectedData := []byte("some sync data")
	expectedETag := "etag-abc"

	t.Run("Data and ETag found", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		rows := sqlmock.NewRows([]string{"user_api_key", "data", "data_etag", "updated_at"}).
			AddRow(apiKey, expectedData, expectedETag, time.Now())

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnRows(rows)

		data, etag, err := repo.GetSyncDataAndETag(ctx, apiKey)
		require.NoError(t, err)
		require.NotNil(t, data)
		require.NotNil(t, etag)
		assert.Equal(t, expectedData, data)
		assert.Equal(t, expectedETag, *etag)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Record not found", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnError(gorm.ErrRecordNotFound)

		data, etag, err := repo.GetSyncDataAndETag(ctx, apiKey)
		require.NoError(t, err)
		assert.Nil(t, data)
		assert.Nil(t, etag)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Database error", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnError(sql.ErrConnDone)

		data, etag, err := repo.GetSyncDataAndETag(ctx, apiKey)
		require.Error(t, err)
		assert.Nil(t, data)
		assert.Nil(t, etag)
		assert.Contains(t, err.Error(), "failed to get sync data and ETag")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSyncRepo_SetSyncData(t *testing.T) {
	ctx := context.Background()
	log := logger.Mock()
	apiKey := "test-api-key-set"
	syncPayload := []byte("new sync data")

	t.Run("Update existing data", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE "user_api_key" = $4`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey).
			WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected
		mock.ExpectCommit()

		newETag, err := repo.SetSyncData(ctx, apiKey, syncPayload)
		require.NoError(t, err)
		require.NotNil(t, newETag)
		assert.True(t, strings.HasPrefix(*newETag, "uuid="), "ETag should start with uuid=")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Insert new data if update fails to find record", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE "user_api_key" = $4`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey).
			WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected by update

		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "sync_data" ("user_api_key","data","data_etag","updated_at") VALUES ($1,$2,$3,$4)`)).
			WithArgs(apiKey, syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1)) // 1 row affected by insert
		mock.ExpectCommit()

		newETag, err := repo.SetSyncData(ctx, apiKey, syncPayload)
		require.NoError(t, err)
		require.NotNil(t, newETag)
		assert.True(t, strings.HasPrefix(*newETag, "uuid="), "ETag should start with uuid=")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Error during update", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE "user_api_key" = $4`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey).
			WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		newETag, err := repo.SetSyncData(ctx, apiKey, syncPayload)
		require.Error(t, err)
		assert.Nil(t, newETag)
		assert.Contains(t, err.Error(), "error updating sync data")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Error during insert after update affected 0 rows", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE "user_api_key" = $4`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey).
			WillReturnResult(sqlmock.NewResult(0, 0))

		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "sync_data" ("user_api_key","data","data_etag","updated_at") VALUES ($1,$2,$3,$4)`)).
			WithArgs(apiKey, syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		newETag, err := repo.SetSyncData(ctx, apiKey, syncPayload)
		require.Error(t, err)
		assert.Nil(t, newETag)
		assert.Contains(t, err.Error(), "error inserting sync data")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Insert affects 0 rows (should be an error)", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE "user_api_key" = $4`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey).
			WillReturnResult(sqlmock.NewResult(0, 0))

		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "sync_data" ("user_api_key","data","data_etag","updated_at") VALUES ($1,$2,$3,$4)`)).
			WithArgs(apiKey, syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		newETag, err := repo.SetSyncData(ctx, apiKey, syncPayload)
		require.Error(t, err)
		assert.Nil(t, newETag)
		assert.Contains(t, err.Error(), "failed to insert sync data, 0 rows affected")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSyncRepo_SetSyncDataIfMatch(t *testing.T) {
	ctx := context.Background()
	log := logger.Mock()
	apiKey := "test-api-key-conditional"
	currentETag := "etag-current-xyz"
	syncPayload := []byte("conditional sync data")

	t.Run("Successful update with matching ETag", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		// Expect the conditional UPDATE statement
		// UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE user_api_key = $4 AND data_etag = $5
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE user_api_key = $4 AND data_etag = $5`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey, currentETag).
			WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected
		mock.ExpectCommit()

		newETag, err := repo.SetSyncDataIfMatch(ctx, apiKey, currentETag, syncPayload)
		require.NoError(t, err)
		require.NotNil(t, newETag)
		assert.True(t, strings.HasPrefix(*newETag, "uuid="), "New ETag should start with uuid=")
		assert.NotEqual(t, currentETag, *newETag, "New ETag should be different from the old one")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ETag mismatch", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE user_api_key = $4 AND data_etag = $5`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey, currentETag).
			WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected due to ETag mismatch or not found

		// Expect a COUNT query to check if the record exists
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(1) // Record exists
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnRows(countRows)
		mock.ExpectCommit() // Or Rollback, depending on GORM's transaction handling for 0 rows affected by update

		newETag, err := repo.SetSyncDataIfMatch(ctx, apiKey, currentETag, syncPayload)
		require.NoError(t, err) // As per implementation, ETag mismatch returns nil, nil
		assert.Nil(t, newETag)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Record not found for conditional update", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE user_api_key = $4 AND data_etag = $5`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey, currentETag).
			WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

		// Expect a COUNT query, record does not exist
		countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnRows(countRows)
		mock.ExpectCommit()

		newETag, err := repo.SetSyncDataIfMatch(ctx, apiKey, currentETag, syncPayload)
		require.NoError(t, err) // As per implementation, not found returns nil, nil
		assert.Nil(t, newETag)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Database error during conditional update", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE user_api_key = $4 AND data_etag = $5`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey, currentETag).
			WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		newETag, err := repo.SetSyncDataIfMatch(ctx, apiKey, currentETag, syncPayload)
		require.Error(t, err)
		assert.Nil(t, newETag)
		assert.Contains(t, err.Error(), "error conditionally updating sync data")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Database error during count check after conditional update failed", func(t *testing.T) {
		db, mock := newMockDB(t)
		repo := NewSyncRepo(log, db)

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "sync_data" SET "data"=$1,"data_etag"=$2,"updated_at"=$3 WHERE user_api_key = $4 AND data_etag = $5`)).
			WithArgs(syncPayload, sqlmock.AnyArg(), sqlmock.AnyArg(), apiKey, currentETag).
			WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "sync_data" WHERE user_api_key = $1`)).
			WithArgs(apiKey).
			WillReturnError(sql.ErrConnDone) // Error during count
		mock.ExpectRollback() // Or Commit, GORM might commit the transaction if the initial update didn't error

		newETag, err := repo.SetSyncDataIfMatch(ctx, apiKey, currentETag, syncPayload)
		// The original code returns nil, nil in this case (line 171 in sync.go)
		// because the countResult.Error is not nil, but it doesn't propagate this error.
		// It logs a warning and returns nil, nil.
		// This might be something to discuss for refactoring, but for now, we test existing behavior.
		require.NoError(t, err)
		assert.Nil(t, newETag)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
