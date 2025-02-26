package workflow

import (
	"context"
)

// Logger interface allows the user of Workflow to provide a custom logger and not use the default which is provided
// in internal/logger. Workflow only writes two types of logs: Debug and Error. Error is only used at the highest
// level where an auto-retry process (consumers and pollers) errors and retries.
//
// Error is used only when the error cannot be passed back to the caller and cannot be bubbled up any further.
//
// Debug is used only when the Workflow is built with WithDebugMode.
type Logger interface {
	// Debug will be used by workflow for debug logs when in debug mode.
	Debug(ctx context.Context, msg string, meta map[string]string)
	// Error is used when writing errors to the logs.
	Error(ctx context.Context, err error)
}

// logger wraps the default logger (internal/logger) or the provided logger (WithLogger) that manages whether a log
// should be written based on the options that the Workflow was built with such as WithDebugMode.
type logger struct {
	debugMode bool
	inner     Logger
}

// Debug only writes the log if the Workflow was built using WithDebugMode
func (l *logger) Debug(ctx context.Context, msg string, meta map[string]string) {
	if !l.debugMode {
		return
	}

	l.inner.Debug(ctx, msg, meta)
}

// Error writes the error to the underlying logger
func (l *logger) Error(ctx context.Context, err error) {
	l.inner.Error(ctx, err)
}
