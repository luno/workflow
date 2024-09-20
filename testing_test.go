package workflow_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestRequire(t *testing.T) {
	b := workflow.NewBuilder[testCustomMarshaler, status]("circular flow")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[testCustomMarshaler, status]) (status, error) {
		*r.Object = "Lower"
		return StatusEnd, nil
	}, StatusEnd)

	wf := b.Build(
		memstreamer.New(),
		memrecordstore.New(),
		memrolescheduler.New(),
	)

	ctx := context.Background()
	wf.Run(ctx)
	t.Cleanup(wf.Stop)

	fid := "10298309123"
	_, err := wf.Trigger(ctx, fid, StatusStart)
	require.Nil(t, err)

	workflow.Require(t, wf, fid, StatusEnd, "Lower")
}

// testCustomMarshaler is for testing and implements custom and weird behaviour via the
// MarshalJSON method and this is to test that the Require function can successfully compare
// the actual and expected by running them through the same encoding / decoding process.
type testCustomMarshaler string

func (t testCustomMarshaler) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.ToLower(string(t)))
}
