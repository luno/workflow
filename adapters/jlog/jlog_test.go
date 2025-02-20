package jlog_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/log"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/jlog"
)

func TestDebug(t *testing.T) {
	buf := bytes.NewBuffer([]byte{})
	jLogger := log.NewCmdLogger(buf, true)
	log.SetLoggerForTesting(t, jLogger)

	logger := jlog.New()
	ctx := context.Background()
	logger.Debug(ctx, "test message", map[string]string{"testKey": "testValue"})

	require.Equal(t, "D 00:00:00.000 g/l/w/a/jlog/jlog.go:19: test message[testkey=testValue]\n", buf.String())
}

func TestError(t *testing.T) {
	buf := bytes.NewBuffer([]byte{})
	jLogger := log.NewCmdLogger(buf, true)
	log.SetLoggerForTesting(t, jLogger)

	logger := jlog.New()
	ctx := context.Background()
	testErr := errors.New("test error")
	logger.Error(ctx, testErr)

	s := buf.String()
	require.True(t, strings.Contains(s, "E 00:00:00.000 g/l/w/a/jlog/jlog.go:23: error(s)"))
	require.True(t, strings.Contains(s, "test error"))
	require.True(t, strings.Contains(s, "github.com/luno/workflow/adapters/jlog/jlog_test.go"))
}
