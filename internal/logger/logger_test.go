package logger_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	werrors "github.com/luno/workflow/internal/errors"
	"github.com/luno/workflow/internal/logger"
)

func TestLoggerDebug(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(&buf)

	ctx := context.Background()
	log.Debug(ctx, "test message", map[string]string{"key": "value"})

	require.Contains(t, buf.String(), "\"level\":\"DEBUG\",\"msg\":\"test message\",\"meta\":{\"key\":\"value\"}")
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(&buf)

	ctx := context.Background()
	log.Error(ctx, errors.New("test error"), map[string]string{"key": "value"})

	require.Contains(t, buf.String(), "\"level\":\"ERROR\",\"msg\":\"test error\",\"meta\":{\"key\":\"value\"}")
}

func TestLogger_InternalError(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(&buf)

	ctx := context.Background()
	log.Error(ctx, werrors.WrapWithMeta(werrors.New("test error"), "", map[string]string{
		"errorMetaKey": "errorMetaValue",
	}), map[string]string{})

	require.Contains(t, buf.String(), "\"level\":\"ERROR\",\"msg\":\"test error\",\"meta\":{\"errorMetaKey\":\"errorMetaValue\"}")
}
