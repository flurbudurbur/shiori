package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"

	"github.com/r3labs/sse/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger interface
type Logger interface {
	Log() *zerolog.Event
	Fatal() *zerolog.Event
	Err(err error) *zerolog.Event
	Error() *zerolog.Event
	Warn() *zerolog.Event
	Info() *zerolog.Event
	Trace() *zerolog.Event
	Debug() *zerolog.Event
	With() zerolog.Context
	RegisterSSEWriter(sse *sse.Server)
	SetLogLevel(level string)
}

// DefaultLogger default logging controller
type DefaultLogger struct {
	log           zerolog.Logger
	level         zerolog.Level
	writers       []io.Writer
	logDir        string
	currentDate   string
	lumberjackLog *lumberjack.Logger
	cfg           *domain.Config
}

func New(cfg *domain.Config) Logger {
	l := &DefaultLogger{
		writers:     make([]io.Writer, 0),
		level:       zerolog.DebugLevel,
		cfg:         cfg,
		currentDate: time.Now().Format("2006-01-02"),
	}

	// set log level
	l.SetLogLevel(cfg.Logging.Level) // Access via nested Logging struct

	// use pretty logging for dev only
	if cfg.Version == "dev" {
		// setup console writer
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}

		l.writers = append(l.writers, consoleWriter)
	} else {
		// default to stderr
		l.writers = append(l.writers, os.Stderr)
	}

	if cfg.Logging.Path != "" { // Access via nested Logging struct
		// Create log directory if it doesn't exist
		l.logDir = cfg.Logging.Path
		if _, err := os.Stat(l.logDir); os.IsNotExist(err) {
			if err := os.MkdirAll(l.logDir, 0755); err != nil {
				fmt.Printf("Failed to create log directory: %v\n", err)
			}
		}

		// Generate filename with current date
		logFilename := filepath.Join(l.logDir, fmt.Sprintf("shiori-%s.log", l.currentDate))

		// Create lumberjack logger
		l.lumberjackLog = &lumberjack.Logger{
			Filename:   logFilename,                // Use date in filename
			MaxSize:    cfg.Logging.MaxFileSize,    // Access via nested Logging struct
			MaxBackups: cfg.Logging.MaxBackupCount, // Access via nested Logging struct
		}

		l.writers = append(l.writers, l.lumberjackLog)
		
		// Start a goroutine to check for log rotation at midnight
		go l.scheduleRotationCheck()
	}

	// set some defaults
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack

	// init new logger
	l.log = zerolog.New(io.MultiWriter(l.writers...)).With().Stack().Logger()

	return l
}

func (l *DefaultLogger) RegisterSSEWriter(sse *sse.Server) {
	w := NewSSEWriter(sse)
	
	// Add the SSE writer to our writers slice
	l.writers = append(l.writers, w)
	
	// Recreate the logger with all writers
	l.log = zerolog.New(io.MultiWriter(l.writers...)).With().Stack().Logger()
	
	// Log that SSE writer was registered
	l.Info().Msg("SSE writer registered for logging")
}

// scheduleRotationCheck runs a timer to check for log rotation at midnight
func (l *DefaultLogger) scheduleRotationCheck() {
	if l.lumberjackLog == nil || l.logDir == "" {
		return // No log file configured
	}

	for {
		// Calculate time until next midnight
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		duration := nextMidnight.Sub(now)

		// Sleep until midnight
		time.Sleep(duration)

		// Perform log rotation
		l.checkRotate()
	}
}

// checkRotate checks if the date has changed and rotates the log file if needed
func (l *DefaultLogger) checkRotate() {
	if l.lumberjackLog == nil || l.logDir == "" {
		return // No log file configured
	}

	today := time.Now().Format("2006-01-02")
	if today != l.currentDate {
		// Date has changed, create a new log file
		l.currentDate = today
		newLogFilename := filepath.Join(l.logDir, fmt.Sprintf("shiori-%s.log", l.currentDate))
		
		// Update the lumberjack logger with the new filename
		l.lumberjackLog.Filename = newLogFilename
		
		// Close the current log file to ensure it's flushed
		_ = l.lumberjackLog.Close()
		
		// Recreate the writers slice with the updated lumberjack logger
		newWriters := make([]io.Writer, 0, len(l.writers))
		for _, w := range l.writers {
			if _, ok := w.(*lumberjack.Logger); ok {
				newWriters = append(newWriters, l.lumberjackLog)
			} else {
				newWriters = append(newWriters, w)
			}
		}
		l.writers = newWriters
		
		// Recreate the logger with the new writers
		l.log = zerolog.New(io.MultiWriter(l.writers...)).With().Stack().Logger()
	}
}

func (l *DefaultLogger) SetLogLevel(level string) {
	switch level {
	case "INFO":
		l.level = zerolog.InfoLevel
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "DEBUG":
		l.level = zerolog.DebugLevel
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "ERROR":
		l.level = zerolog.ErrorLevel
	case "WARN":
		l.level = zerolog.WarnLevel
	case "TRACE":
		l.level = zerolog.TraceLevel
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	default:
		l.level = zerolog.Disabled
	}
}

// Log log something at fatal level.
func (l *DefaultLogger) Log() *zerolog.Event {
	l.checkRotate()
	return l.log.Log().Timestamp()
}

// Fatal log something at fatal level. This will panic!
func (l *DefaultLogger) Fatal() *zerolog.Event {
	l.checkRotate()
	return l.log.Fatal().Timestamp()
}

// Error log something at Error level
func (l *DefaultLogger) Error() *zerolog.Event {
	l.checkRotate()
	return l.log.Error().Timestamp()
}

// Err log something at Err level
func (l *DefaultLogger) Err(err error) *zerolog.Event {
	l.checkRotate()
	return l.log.Err(err).Timestamp()
}

// Warn log something at warning level.
func (l *DefaultLogger) Warn() *zerolog.Event {
	l.checkRotate()
	return l.log.Warn().Timestamp()
}

// Info log something at fatal level.
func (l *DefaultLogger) Info() *zerolog.Event {
	l.checkRotate()
	return l.log.Info().Timestamp()
}

// Debug log something at debug level.
func (l *DefaultLogger) Debug() *zerolog.Event {
	l.checkRotate()
	return l.log.Debug().Timestamp()
}

// Trace log something at fatal level. This will panic!
func (l *DefaultLogger) Trace() *zerolog.Event {
	l.checkRotate()
	return l.log.Trace().Timestamp()
}

// With log with context
func (l *DefaultLogger) With() zerolog.Context {
	l.checkRotate()
	return l.log.With().Timestamp()
}
