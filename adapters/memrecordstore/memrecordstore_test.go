package memrecordstore_test

import (
	"testing"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/memrecordstore"
	connectortesting "github.com/andrewwormald/workflow/adapters/testing"
)

func TestStore(t *testing.T) {
	connectortesting.TestRecordStore(t, func() workflow.RecordStore {
		return memrecordstore.New()
	})
}
