package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopRunStateController(t *testing.T) {
	ctrl := noopRunStateController{}

	ctx := context.Background()
	err := ctrl.Pause(ctx)
	require.Nil(t, err)

	err = ctrl.Resume(ctx)
	require.Nil(t, err)

	err = ctrl.Cancel(ctx)
	require.Nil(t, err)

	err = ctrl.DeleteData(ctx)
	require.Nil(t, err)
}
