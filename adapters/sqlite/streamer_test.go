package sqlite_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"

	"github.com/luno/workflow/adapters/sqlite"
)

func TestEventStreamer(t *testing.T) {
	adaptertest.RunEventStreamerTest(t, func() workflow.EventStreamer {
		db := connectForTesting(t)
		return sqlite.NewEventStreamer(db)
	})
}
