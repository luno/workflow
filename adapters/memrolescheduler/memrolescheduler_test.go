package memrolescheduler_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memrolescheduler"
)

func TestRoleScheduler(t *testing.T) {
	adaptertest.RunRoleSchedulerTest(t, func() workflow.RoleScheduler {
		return memrolescheduler.New()
	})
}
