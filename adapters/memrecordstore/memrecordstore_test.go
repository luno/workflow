package memrecordstore_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	connectortesting "github.com/luno/workflow/adapters/testing"
)

func TestStore(t *testing.T) {
	connectortesting.TestRecordStore(t, func() workflow.RecordStore {
		return memrecordstore.New()
	})
}
