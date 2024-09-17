package workflow_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
)

func TestCreateDiagram(t *testing.T) {
	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle, StatusEnd)

	b.AddStep(StatusMiddle, func(ctx context.Context, r *workflow.Run[string, status]) (status, error) {
		return StatusEnd, nil
	}, StatusEnd,
	)

	wf := b.Build(nil, nil, nil)

	err := workflow.CreateDiagram(wf, "./testdata/graph-visualisation.md", workflow.LeftToRightDirection)
	jtest.RequireNil(t, err)
}

func TestCreateDiagramValidation(t *testing.T) {
	w := (workflow.API[string, status])(nil)
	err := workflow.CreateDiagram(w, "./testdata/should-not-exist.md", workflow.LeftToRightDirection)
	require.Equal(t, err.Error(), "cannot create diagram for non-original workflow.Workflow type")
}
