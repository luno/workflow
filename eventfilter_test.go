package workflow

import (
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/luno/jettison/jtest"
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

func TestShardNonNumerical(t *testing.T) {
	fn := shardConnectorEventFilter(1, 2)

	var (
		left, right []string
		total       = 1000
	)
	for i := 0; i < total; i++ {
		uid, err := uuid.NewUUID()
		jtest.RequireNil(t, err)

		e := &ConnectorEvent{
			ID: uid.String(),
		}
		if fn(e) {
			left = append(left, e.ID)
		} else {
			right = append(right, e.ID)
		}
	}

	leftProportion := float64(len(left)) / float64(total)
	rightProportion := float64(len(right)) / float64(total)
	lowerThresholdOfDistribution := 0.4
	require.Greater(t, leftProportion, lowerThresholdOfDistribution)
	require.Greater(t, rightProportion, lowerThresholdOfDistribution)
	upperThresholdOfDistribution := 0.6
	require.Less(t, leftProportion, upperThresholdOfDistribution)
	require.Less(t, rightProportion, upperThresholdOfDistribution)
}

func TestConnectorShardFilter(t *testing.T) {
	ids := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22"}
	totalShards := 2
	shardLeft := 1
	shardRight := 2

	right := make(map[string]bool)
	left := make(map[string]bool)
	for _, id := range ids {
		isNotRight := shardConnectorEventFilter(shardRight, totalShards)
		isNotLeft := shardConnectorEventFilter(shardLeft, totalShards)

		e := &ConnectorEvent{
			ID: id,
		}

		if isNotRight(e) {
			left[id] = true
		} else if isNotLeft(e) {
			right[id] = true
		}
	}

	expectedRight := map[string]bool{
		"1":  true,
		"3":  true,
		"5":  true,
		"7":  true,
		"9":  true,
		"10": true,
		"12": true,
		"14": true,
		"16": true,
		"18": true,
		"21": true,
	}
	require.Equal(t, expectedRight, right)

	expectedLeft := map[string]bool{
		"2":  true,
		"4":  true,
		"6":  true,
		"8":  true,
		"11": true,
		"13": true,
		"15": true,
		"17": true,
		"19": true,
		"20": true,
		"22": true,
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
