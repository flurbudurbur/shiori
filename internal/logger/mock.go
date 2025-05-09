package logger

import (
	"github.com/rs/zerolog"
	"io"
)

func Mock() Logger {
	l := &DefaultLogger{
		writers:     make([]io.Writer, 0),
		level:       zerolog.Disabled,
		currentDate: "2006-01-02", // Set a default date to avoid nil pointer dereferences
	}

	// init new logger
	l.log = zerolog.New(io.MultiWriter(l.writers...)).With().Stack().Logger()

	return l
}
