package memrecordstore_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memrecordstore"
)

func TestStore(t *testing.T) {
	adaptertest.RunRecordStoreTest(t, func() workflow.RecordStore {
		return memrecordstore.New()
	})
}
