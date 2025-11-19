package sqlite_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"

	"github.com/luno/workflow/adapters/sqlite"
)

func TestTimeoutStore(t *testing.T) {
	adaptertest.RunTimeoutStoreTest(t, func() workflow.TimeoutStore {
		db := connectForTesting(t)
		return sqlite.NewTimeoutStore(db)
	})
}
