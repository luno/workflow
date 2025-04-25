package logger_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/logger"
)

func TestLoggerDebug(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(&buf)

	ctx := t.Context()
	log.Debug(ctx, "test message", map[string]string{"key": "value"})

	require.Contains(t, buf.String(), "\"level\":\"DEBUG\",\"msg\":\"test message\",\"meta\":{\"key\":\"value\"}")
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(&buf)

	ctx := t.Context()
	log.Error(ctx, errors.New("test error"))

	require.Contains(t, buf.String(), "\"level\":\"ERROR\",\"msg\":\"test error\"")
}
