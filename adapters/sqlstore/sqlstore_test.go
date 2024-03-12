package sqlstore_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/sqlstore"
)

func TestStore(t *testing.T) {
	adaptertest.RunRecordStoreTest(t, func() workflow.RecordStore {
		dbc := ConnectForTesting(t)
		return sqlstore.New(dbc, dbc, "workflow_records", "workflow_outbox")
	})
}
