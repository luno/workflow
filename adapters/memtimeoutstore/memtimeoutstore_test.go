package memtimeoutstore_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memtimeoutstore"
)

func TestStore(t *testing.T) {
	adaptertest.RunTimeoutStoreTest(t, func() workflow.TimeoutStore {
		return memtimeoutstore.New()
	})
}
