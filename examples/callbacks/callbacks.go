package callbacks

import (
	"context"
	"encoding/json"
	"io"

	"github.com/luno/workflow"
	"github.com/luno/workflow/examples"
)

type Example struct {
	EmailConfirmed bool
}

type EmailConfirmationResponse struct {
	Confirmed bool
}

type Deps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	TimeoutStore  workflow.TimeoutStore
	RoleScheduler workflow.RoleScheduler
}

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, examples.Status] {
	b := workflow.NewBuilder[Example, examples.Status]("callback example")

	b.AddCallback(examples.StatusStarted, func(ctx context.Context, r *workflow.Record[Example, examples.Status], reader io.Reader) (examples.Status, error) {
		b, err := io.ReadAll(reader)
		if err != nil {
			return 0, err
		}

		var e EmailConfirmationResponse
		err = json.Unmarshal(b, &e)
		if err != nil {
			return 0, err
		}

		r.Object.EmailConfirmed = e.Confirmed

		return examples.StatusFollowedTheExample, nil
	}, examples.StatusFollowedTheExample)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.TimeoutStore,
		d.RoleScheduler,
	)
}
