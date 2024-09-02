package errors_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	werrors "github.com/luno/workflow/internal/errors"
)

func TestError_Is(t *testing.T) {
	testErr := errors.New("level 1")
	err := werrors.Wrap(testErr, "level 2")
	require.True(t, errors.Is(err, testErr))

	e, ok := err.(*werrors.Error)
	require.True(t, ok)
	require.False(t, e.Is(nil))
}

func TestWrappingNil(t *testing.T) {
	require.Nil(t, werrors.Wrap(nil, ""))
	require.Nil(t, werrors.WrapWithMeta(nil, "", map[string]string{}))
}

func TestNewBlankMsgNilError(t *testing.T) {
	require.Nil(t, werrors.New(""))
}

func TestWrap(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		msg      string
		expected string
	}{
		{
			name:     "Test error wrapped with message",
			err:      errors.New("test error"),
			msg:      "",
			expected: "test error",
		},
		{
			name:     "Test error wrapped with message",
			err:      werrors.New("test error"),
			msg:      "test",
			expected: "test\ntest error",
		},
		{
			name:     "Test error wrapped with message",
			err:      werrors.Wrap(werrors.New("level 1"), "level 2"),
			msg:      "level 3",
			expected: "level 3\nlevel 2\nlevel 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := werrors.Wrap(tc.err, tc.msg)
			require.Equal(t, tc.expected, err.Error())
		})
	}
}

func TestWrapWithMeta(t *testing.T) {
	testCases := []struct {
		name          string
		err           error
		msg           string
		meta          map[string]string
		expectedError string
		expectedMeta  map[string]string
	}{
		{
			name:          "Test error wrapped with message and meta",
			err:           errors.New("test error"),
			msg:           "test",
			meta:          map[string]string{"key1": "value1", "key2": "value2"},
			expectedError: "test\ntest error",
			expectedMeta:  map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "Test wrapped error with message and wrapped meta",
			err: werrors.WrapWithMeta(werrors.New("level 1"), "level 2", map[string]string{
				"wrappedKey": "wrappedValue",
			}),
			msg:           "level 3",
			meta:          map[string]string{"key1": "value1", "key2": "value2"},
			expectedError: "level 3\nlevel 2\nlevel 1",
			expectedMeta:  map[string]string{"key1": "value1", "key2": "value2", "wrappedKey": "wrappedValue"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := werrors.WrapWithMeta(tc.err, tc.msg, tc.meta)
			require.Equal(t, tc.expectedError, err.Error())

			e, ok := err.(*werrors.Error)
			require.True(t, ok)
			require.Equal(t, tc.expectedMeta, e.Meta())
		})
	}
}
