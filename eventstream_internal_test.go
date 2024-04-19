package workflow

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShardFilter(t *testing.T) {
	ids := []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	totalShards := 2
	shardLeft := 1
	shardRight := 2

	right := make(map[int64]bool)
	left := make(map[int64]bool)
	for _, id := range ids {
		isNotRight := shardFilter(shardRight, totalShards)
		isNotLeft := shardFilter(shardLeft, totalShards)

		e := &Event{
			ID: id,
		}

		if isNotRight(e) {
			left[id] = true
		} else if isNotLeft(e) {
			right[id] = true
		}
	}

	expectedRight := map[int64]bool{
		1: true,
		3: true,
		5: true,
		7: true,
		9: true,
	}
	require.Equal(t, expectedRight, right)

	expectedLeft := map[int64]bool{
		2:  true,
		4:  true,
		6:  true,
		8:  true,
		10: true,
	}
	require.Equal(t, expectedLeft, left)
}

func TestRunStateUpdatesFilter(t *testing.T) {
	testCases := []struct {
		name     string
		previous RunState
		current  RunState
		expected bool
	}{
		{
			name:     "Initiated -> Running",
			previous: RunStateInitiated,
			current:  RunStateRunning,
			expected: true,
		},
		{
			name:     "Running -> Running",
			previous: RunStateRunning,
			current:  RunStateRunning,
			expected: false,
		},
		{
			name:     "Running -> Completed",
			previous: RunStateRunning,
			current:  RunStateCompleted,
			expected: false,
		},
		{
			name:     "Completed -> DataDeleted",
			previous: RunStateCompleted,
			current:  RunStateDataDeleted,
			expected: true,
		},
		{
			name:     "Running -> Cancelled",
			previous: RunStateRunning,
			current:  RunStateCancelled,
			expected: true,
		},
		{
			name:     "Running -> Paused",
			previous: RunStateRunning,
			current:  RunStatePaused,
			expected: true,
		},
		{
			name:     "Paused -> Running",
			previous: RunStatePaused,
			current:  RunStateRunning,
			expected: false,
		},
		{
			name:     "Paused -> Cancelled",
			previous: RunStatePaused,
			current:  RunStateCancelled,
			expected: true,
		},
		{
			name:     "Cancelled -> DataDeleted",
			previous: RunStateCancelled,
			current:  RunStateDataDeleted,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			e := &Event{
				Headers: map[Header]string{
					HeaderRunState:         strconv.FormatInt(int64(tc.current), 10),
					HeaderPreviousRunState: strconv.FormatInt(int64(tc.previous), 10),
				},
			}
			filter := runStateUpdatesFilter()

			require.Equal(t, tc.expected, filter(e))
		})
	}
}
