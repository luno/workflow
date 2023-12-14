package kafka

import (
	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/kafkastreamer"
	"github.com/andrewwormald/workflow/adapters/memrecordstore"
	"github.com/andrewwormald/workflow/adapters/memrolescheduler"
	"github.com/andrewwormald/workflow/adapters/memtimeoutstore"
	"github.com/andrewwormald/workflow/examples"
	"github.com/andrewwormald/workflow/examples/gettingstarted"
)

func ExampleWorkflow() *workflow.Workflow[gettingstarted.GettingStarted, examples.Status] {
	return gettingstarted.WorkflowWithEnum(gettingstarted.Deps{
		EventStreamer: kafkastreamer.New([]string{"localhost:9092"}),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
}
