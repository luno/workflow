package workflow_test

import (
	"context"
	"testing"

	"github.com/luno/jettison/jtest"

	"github.com/luno/workflow"
)

func TestVisualiser(t *testing.T) {
	b := workflow.NewBuilder[string, status]("example")
	b.AddStep(StatusStart, func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
		return StatusMiddle, nil
	}, StatusMiddle, StatusEnd)

	b.AddStep(StatusMiddle,
		func(ctx context.Context, r *workflow.Record[string, status]) (status, error) {
			return StatusEnd, nil
		}, StatusEnd,
	)

	wf := b.Build(nil, nil, nil, nil)

	err := workflow.MermaidDiagram(wf, "./testfiles/testgraph.md", workflow.LeftToRightDirection)
	jtest.RequireNil(t, err)
}
