package sqltimeout_test

import (
	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/sqltimeout"
	"testing"

	adapterstesting "github.com/andrewwormald/workflow/adapters/testing"
)

func TestStore(t *testing.T) {
	adapterstesting.TestTimeoutStore(t, func() workflow.TimeoutStore {
		dbc := ConnectForTesting(t)
		return sqltimeout.New(dbc, dbc, "workflow_timeouts")
	})
}
