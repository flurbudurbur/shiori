package logger

import (
	"testing"
)

func TestMockLogger(t *testing.T) {
	logger := Mock()
	if logger == nil {
		t.Fatal("Mock() returned nil")
	}
	// Should not panic when calling methods
	logger.Log()
	logger.Fatal()
	logger.Error()
	logger.Err(nil)
	logger.Warn()
	logger.Info()
	logger.Debug()
	logger.Trace()
	logger.With()
}