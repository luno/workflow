package callback

import (
	"context"
	"encoding/json"
	"io"

	"github.com/luno/workflow"
)

type Status int

var (
	StatusUnknown            Status = 0
	StatusStarted            Status = 1
	StatusReadTheDocs        Status = 2
	StatusFollowedTheExample Status = 3
	StatusCreatedAFunExample Status = 4
)

func (s Status) String() string {
	switch s {
	case StatusStarted:
		return "Started"
	case StatusReadTheDocs:
		return "Read the docs"
	case StatusFollowedTheExample:
		return "Followed the example"
	case StatusCreatedAFunExample:
		return "Created a fun example"
	default:
		return "Unknown"
	}
}

type Example struct {
	EmailConfirmed bool
}

type EmailConfirmationResponse struct {
	Confirmed bool
}

type Deps struct {
	EventStreamer workflow.EventStreamer
	RecordStore   workflow.RecordStore
	RoleScheduler workflow.RoleScheduler
}

func ExampleWorkflow(d Deps) *workflow.Workflow[Example, Status] {
	b := workflow.NewBuilder[Example, Status]("callback example")

	b.AddCallback(StatusStarted, func(ctx context.Context, r *workflow.Run[Example, Status], reader io.Reader) (Status, error) {
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

		return StatusFollowedTheExample, nil
	}, StatusFollowedTheExample)

	return b.Build(
		d.EventStreamer,
		d.RecordStore,
		d.RoleScheduler,
	)
}
