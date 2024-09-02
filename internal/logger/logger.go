package logger

import (
	"context"
	"io"
	"log/slog"

	"github.com/luno/workflow/internal/errors"
)

type logger struct {
	log *slog.Logger
}

func (l logger) Debug(ctx context.Context, msg string, meta map[string]string) {
	l.log.DebugContext(ctx, msg, "meta", meta)
}

func (l logger) Error(ctx context.Context, err error, meta map[string]string) {
	e, isInternalError := err.(*errors.Error)
	if isInternalError {
		for k, v := range e.Meta() {
			meta[k] = v
		}
	}

	l.log.ErrorContext(ctx, err.Error(), "meta", meta)
}

func New(w io.Writer) *logger {
	// LevelDebug is set by default as the workflow has a debug configuration.
	opts := slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	sl := slog.New(slog.NewJSONHandler(w, &opts))
	return &logger{
		log: sl,
	}
}
