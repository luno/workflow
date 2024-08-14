package sqltimeout_test

import (
	"github.com/luno/workflow/adapters/sqltimeout"
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
)

func TestStore(t *testing.T) {
	adaptertest.RunTimeoutStoreTest(t, func() workflow.TimeoutStore {
		dbc := ConnectForTesting(t)
		return sqltimeout.New(dbc, dbc, "workflow_timeouts")
	})
}
