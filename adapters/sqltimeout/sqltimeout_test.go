package sqltimeout_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
<<<<<<< Updated upstream
=======

>>>>>>> Stashed changes
	"github.com/luno/workflow/adapters/sqltimeout"
)

func TestStore(t *testing.T) {
	adaptertest.RunTimeoutStoreTest(t, func() workflow.TimeoutStore {
		dbc := ConnectForTesting(t)
		return sqltimeout.New(dbc, dbc, "workflow_timeouts")
	})
}
