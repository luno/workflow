package workflow

import "context"

type Logger interface {
	// Debug will be used by workflow for debug logs when in debug mode.
	Debug(ctx context.Context, msg string, meta map[string]string)
	// Error is used when writing errors to the logs.
	Error(ctx context.Context, err error, meta map[string]string)
}

// MKV is alias for ma[string]string to simplify the passing of Multiple Key Values to the logger.
type MKV map[string]string
