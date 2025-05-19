//go:build !integration

package logger

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/r3labs/sse/v2"
	"github.com/rs/zerolog"
)


func TestNewLogger_Defaults(t *testing.T) {
	cfg := &domain.Config{
		Version: "dev",
		Logging: domain.LoggingConfig{
			Level:         "DEBUG",
			Path:          "",
			MaxFileSize:   1,
			MaxBackupCount: 1,
		},
	}
	logger := New(cfg)
	if logger == nil {
		t.Fatal("Expected logger to be non-nil")
	}
}

func TestNewLogger_LogDirCreation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &domain.Config{
		Version: "prod",
		Logging: domain.LoggingConfig{
			Level:         "INFO",
			Path:          tmpDir,
			MaxFileSize:   1,
			MaxBackupCount: 1,
		},
	}
	logger := New(cfg)
	l, ok := logger.(*DefaultLogger)
	if !ok {
		t.Fatal("Expected DefaultLogger type")
	}
	if l.logDir != tmpDir {
		t.Errorf("Expected logDir %s, got %s", tmpDir, l.logDir)
	}
	if l.lumberjackLog == nil {
		t.Error("Expected lumberjackLog to be initialized")
	}
}

func TestSetLogLevel(t *testing.T) {
	cfg := &domain.Config{
		Version: "dev",
		Logging: domain.LoggingConfig{Level: "DEBUG"},
	}
	l := New(cfg).(*DefaultLogger)
	levels := []struct {
		input string
		want  zerolog.Level
	}{
		{"INFO", zerolog.InfoLevel},
		{"DEBUG", zerolog.DebugLevel},
		{"ERROR", zerolog.ErrorLevel},
		{"WARN", zerolog.WarnLevel},
		{"TRACE", zerolog.TraceLevel},
		{"INVALID", zerolog.Disabled},
	}
	for _, tc := range levels {
		l.SetLogLevel(tc.input)
		if l.level != tc.want {
			t.Errorf("SetLogLevel(%q): got %v, want %v", tc.input, l.level, tc.want)
		}
	}
}

func TestLoggerMethods(t *testing.T) {
	cfg := &domain.Config{
		Version: "dev",
		Logging: domain.LoggingConfig{Level: "DEBUG"},
	}
	l := New(cfg).(*DefaultLogger)
	_ = l.Log()
	_ = l.Fatal()
	_ = l.Error()
	_ = l.Err(errors.New("test"))
	_ = l.Warn()
	_ = l.Info()
	_ = l.Debug()
	_ = l.Trace()
	_ = l.With()
}

func TestRegisterSSEWriter(t *testing.T) {
	cfg := &domain.Config{
		Version: "dev",
		Logging: domain.LoggingConfig{Level: "DEBUG"},
	}
	l := New(cfg).(*DefaultLogger)
	sse := &sse.Server{}
	l.RegisterSSEWriter(sse)
	// No panic or error expected
}

func TestCheckRotate_NoLogFile(t *testing.T) {
	cfg := &domain.Config{
		Version: "dev",
		Logging: domain.LoggingConfig{Level: "DEBUG"},
	}
	l := New(cfg).(*DefaultLogger)
	l.lumberjackLog = nil
	l.logDir = ""
	l.checkRotate() // Should not panic or error
}

func TestCheckRotate_Rotation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &domain.Config{
		Version: "prod",
		Logging: domain.LoggingConfig{
			Level:         "INFO",
			Path:          tmpDir,
			MaxFileSize:   1,
			MaxBackupCount: 1,
		},
	}
	l := New(cfg).(*DefaultLogger)
	// Simulate a day change
	l.currentDate = "2000-01-01"
	l.checkRotate()
	expected := filepath.Join(tmpDir, time.Now().Format("2006-01-02")+".log")
	if l.lumberjackLog.Filename != expected && !filepath.HasPrefix(l.lumberjackLog.Filename, tmpDir) {
		t.Errorf("Expected rotated log filename to be in %s, got %s", tmpDir, l.lumberjackLog.Filename)
	}
}

func TestScheduleRotationCheck_NoLogFile(t *testing.T) {
	cfg := &domain.Config{
		Version: "dev",
		Logging: domain.LoggingConfig{Level: "DEBUG"},
	}
	l := New(cfg).(*DefaultLogger)
	l.lumberjackLog = nil
	l.logDir = ""
	done := make(chan struct{})
	go func() {
		l.scheduleRotationCheck() // Should return immediately
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("scheduleRotationCheck did not return as expected")
	}
}