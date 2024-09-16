package jlog_test

import (
	"bytes"
	"context"
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
	logger.Error(ctx, testErr, map[string]string{"testKey": "testValue"})

	expected := `E 00:00:00.000 g/l/w/a/jlog/jlog.go:23: error(s) [testkey=testValue]
  test error
  - github.com/luno/workflow/adapters/jlog/jlog_test.go:34 TestError
  - testing/testing.go:1689 tRunner
  - runtime/asm_arm64.s:1222 goexit
`
	require.Equal(t, expected, buf.String())
}
