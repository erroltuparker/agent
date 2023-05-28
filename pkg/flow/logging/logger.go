package logging

import (
	"fmt"
	"io"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Logger implements the github.com/go-kit/log.Logger interface. It supports
// being dynamically updated at runtime.
type Logger struct {
	w io.Writer

	mut sync.RWMutex
	l   log.Logger
}

// New creates a New logger with the default log level and format.
func New(w io.Writer, o Options) (*Logger, error) {
	inner, err := buildLogger(w, o)
	if err != nil {
		return nil, err
	}

	return &Logger{w: w, l: inner}, nil
}

// Log implements log.Logger.
func (l *Logger) Log(kvps ...interface{}) error {
	l.mut.RLock()
	defer l.mut.RUnlock()
	return l.l.Log(kvps...)
}

// Update re-configures the options used for the logger.
func (l *Logger) Update(o Options) error {
	newLogger, err := buildLogger(l.w, o)
	if err != nil {
		return err
	}

	l.mut.Lock()
	defer l.mut.Unlock()
	l.l = newLogger
	return nil
}

func buildLogger(w io.Writer, o Options) (log.Logger, error) {
	var l log.Logger

	switch o.Format {
	case FormatLogfmt:
		l = log.NewLogfmtLogger(log.NewSyncWriter(w))
	case FormatJSON:
		l = log.NewJSONLogger(log.NewSyncWriter(w))
	default:
		return nil, fmt.Errorf("unrecognized log format %q", o.Format)
	}

	l = level.NewFilter(l, o.Level.Filter())

	l = log.With(l, "ts", log.DefaultTimestampUTC)
	return l, nil
}
