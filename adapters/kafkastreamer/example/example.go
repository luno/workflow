package example

import (
	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	"github.com/luno/workflow/example/gettingstarted"

	"github.com/luno/workflow/adapters/kafkastreamer"
)

func ExampleWorkflow() *workflow.Workflow[gettingstarted.GettingStarted, _example.Status] {
	return gettingstarted.Workflow(gettingstarted.Deps{
		EventStreamer: kafkastreamer.New([]string{"localhost:9092"}),
		RecordStore:   memrecordstore.New(),
		TimeoutStore:  memtimeoutstore.New(),
		RoleScheduler: memrolescheduler.New(),
	})
}
