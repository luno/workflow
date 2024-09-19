package memrolescheduler_test

import (
	"github.com/luno/workflow/adapters/memrolescheduler"
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
)

func TestRoleScheduler(t *testing.T) {
	adaptertest.RunRoleSchedulerTest(t, func(t *testing.T, instances int) []workflow.RoleScheduler {
		singleton := memrolescheduler.New()

		var rs []workflow.RoleScheduler
		for range instances {
			rs = append(rs, singleton)
		}

		return rs
	})
}
