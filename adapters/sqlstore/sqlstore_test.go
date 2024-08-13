package sqlstore_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
)

func TestStore(t *testing.T) {
	adaptertest.RunRecordStoreTest(t, func() workflow.RecordStore {
		dbc := ConnectForTesting(t)
		return New(dbc, dbc, "workflow_records", "workflow_outbox")
	})
}
