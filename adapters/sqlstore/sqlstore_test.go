package sqlstore_test

import (
	"testing"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/sqlstore"
	connectorstesting "github.com/andrewwormald/workflow/adapters/testing"
)

func TestStore(t *testing.T) {
	connectorstesting.TestRecordStore(t, func() workflow.RecordStore {
		dbc := ConnectForTesting(t)
		return sqlstore.New(dbc, dbc, "workflow_records")
	})
}
