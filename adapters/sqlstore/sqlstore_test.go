package sqlstore_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/sqlstore"
	connectorstesting "github.com/luno/workflow/adapters/testing"
)

func TestStore(t *testing.T) {
	connectorstesting.TestRecordStore(t, func() workflow.RecordStore {
		dbc := ConnectForTesting(t)
		return sqlstore.New(dbc, dbc, "workflow_records")
	})
}
