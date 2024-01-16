package sqltimeout_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/sqltimeout"
	adapterstesting "github.com/luno/workflow/adapters/testing"
)

func TestStore(t *testing.T) {
	adapterstesting.TestTimeoutStore(t, func() workflow.TimeoutStore {
		dbc := ConnectForTesting(t)
		return sqltimeout.New(dbc, dbc, "workflow_timeouts")
	})
}
