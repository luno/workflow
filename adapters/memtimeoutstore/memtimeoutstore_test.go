package memtimeoutstore_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memtimeoutstore"
	connectortesting "github.com/luno/workflow/adapters/testing"
)

func TestStore(t *testing.T) {
	connectortesting.TestTimeoutStore(t, func() workflow.TimeoutStore {
		return memtimeoutstore.New()
	})
}
