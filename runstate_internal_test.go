package workflow

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"
)

func TestNoopRunStateController(t *testing.T) {
	ctrl := noopRunStateController{}

	ctx := context.Background()
	err := ctrl.Pause(ctx)
	jtest.RequireNil(t, err)

	err = ctrl.Resume(ctx)
	jtest.RequireNil(t, err)

	err = ctrl.Cancel(ctx)
	jtest.RequireNil(t, err)

	err = ctrl.DeleteData(ctx)
	jtest.RequireNil(t, err)

	err = ctrl.markAsRunning(ctx)
	jtest.RequireNil(t, err)
}
