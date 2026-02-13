package workflow

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	testingclock "k8s.io/utils/clock/testing"
)

// BenchmarkMakeRole benchmarks the current makeRole implementation
func BenchmarkMakeRole(b *testing.B) {
	inputs := []string{"workflow", "status123", "consumer", "1", "of", "10"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = makeRole(inputs...)
	}
}

// BenchmarkTopicFunctions benchmarks topic name generation
func BenchmarkTopicFunctions(b *testing.B) {
	b.Run("Topic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = Topic("my-workflow-name", 123)
		}
	})
	
	b.Run("DeleteTopic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = DeleteTopic("my-workflow-name")
		}
	})
	
	b.Run("RunStateChangeTopic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = RunStateChangeTopic("my-workflow-name")
		}
	})
}

// BenchmarkMetricLabels benchmarks metric label creation
func BenchmarkMetricLabels(b *testing.B) {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_counter",
			Help: "Test counter",
		},
		[]string{"workflow", "process"},
	)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.WithLabelValues("my-workflow", "my-process").Inc()
	}
}

// BenchmarkMetricLabelsCached benchmarks using cached metric labels
func BenchmarkMetricLabelsCached(b *testing.B) {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_counter_cached",
			Help: "Test counter",
		},
		[]string{"workflow", "process"},
	)
	
	// Cache the labeled counter
	cachedCounter := counter.WithLabelValues("my-workflow", "my-process")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cachedCounter.Inc()
	}
}

// BenchmarkTimerAllocation benchmarks timer allocation patterns
func BenchmarkTimerAllocation(b *testing.B) {
	ctx := context.Background()
	clk := testingclock.NewFakeClock(time.Now())
	
	b.Run("NewTimerEachTime", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			delay := time.Millisecond
			if delay > 0 {
				t := clk.NewTimer(delay)
				select {
				case <-ctx.Done():
				case <-t.C():
				}
			}
		}
	})
	
	b.Run("SkipTimerWhenNotNeeded", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			delay := time.Duration(0)
			if delay > 0 {
				t := clk.NewTimer(delay)
				select {
				case <-ctx.Done():
				case <-t.C():
				}
			}
		}
	})
}

// BenchmarkPushLagMetric benchmarks the lag metric push
func BenchmarkPushLagMetric(b *testing.B) {
	workflowName := "test-workflow"
	processName := "test-process"
	clk := testingclock.NewFakeClock(time.Now())
	timestamp := clk.Now().Add(-time.Second)
	lagThreshold := 5 * time.Second
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pushLagMetricAndAlerting(workflowName, processName, timestamp, lagThreshold, clk)
	}
}

// BenchmarkStringOperations benchmarks common string conversion approaches
func BenchmarkStringOperations(b *testing.B) {
	b.Run("FormatInt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = strconv.FormatInt(int64(i), 10)
		}
	})
	
	b.Run("Itoa", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = strconv.Itoa(i)
		}
	})
}

// BenchmarkMapAllocation benchmarks map allocation patterns
func BenchmarkMapAllocation(b *testing.B) {
	b.Run("NoCapacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m := make(map[string]string)
			m["a"] = "1"
			m["b"] = "2"
			m["c"] = "3"
			m["d"] = "4"
			m["e"] = "5"
			m["f"] = "6"
		}
	})
	
	b.Run("WithCapacity", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m := make(map[string]string, 6)
			m["a"] = "1"
			m["b"] = "2"
			m["c"] = "3"
			m["d"] = "4"
			m["e"] = "5"
			m["f"] = "6"
		}
	})
}
