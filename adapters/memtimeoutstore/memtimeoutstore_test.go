package memtimeoutstore_test

import (
	"testing"

	"github.com/andrewwormald/workflow"
	"github.com/andrewwormald/workflow/adapters/memtimeoutstore"
	connectortesting "github.com/andrewwormald/workflow/adapters/testing"
)

func TestStore(t *testing.T) {
	connectortesting.TestTimeoutStore(t, func() workflow.TimeoutStore {
		return memtimeoutstore.New()
	})
}
