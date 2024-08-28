package workflow

import "context"

type Logger interface {
	// Debug will be used by workflow for debug logs when in debug mode.
	Debug(ctx context.Context, msg string, meta MKV)
	// Error is used when writing errors to the logs.
	Error(ctx context.Context, err error)
}

// MKV is a multiple key value store for the logger to format into its output.
type MKV map[string]string
