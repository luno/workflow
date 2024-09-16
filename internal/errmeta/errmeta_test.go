package errmeta_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/internal/errmeta"
)

func TestErrMeta(t *testing.T) {
	testErr := errors.New("test error")
	err := errmeta.New(testErr, map[string]string{"food": "🍣"})

	t.Run("Compatible with errors.Is", func(t *testing.T) {
		errors.Is(err, testErr)
	})

	t.Run("Meta data is extractable", func(t *testing.T) {
		var asErr *errmeta.ErrMeta
		require.True(t, errors.As(err, &asErr))
		require.Equal(t, "🍣", asErr.Meta()["food"])
	})

	t.Run("Matches error string", func(t *testing.T) {
		require.Equal(t, testErr.Error(), err.Error())
	})
}
