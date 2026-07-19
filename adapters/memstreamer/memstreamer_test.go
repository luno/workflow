package memstreamer_test

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memstreamer"
)

const testTopic = "test-topic"

func TestStreamer(t *testing.T) {
	adaptertest.RunEventStreamerTest(t, func() workflow.EventStreamer {
		return memstreamer.New()
	})
}

func TestConnector(t *testing.T) {
	adaptertest.RunConnectorTest(t, func(seedEvents []workflow.ConnectorEvent) workflow.ConnectorConstructor {
		return memstreamer.NewConnector(seedEvents)
	})
}

// TestRecv_DoesNotBusySpin verifies the headline fix: with several receivers
// parked and no senders, the goroutine count stays flat AND the receivers
// are sleeping on sync.Cond rather than spinning runnable. The busy-loop
// implementation kept goroutine count stable too, so the test also inspects
// goroutine stacks to confirm they are parked on cond.Wait (semaphore wait)
// rather than in the middle of the Recv for-loop.
func TestRecv_DoesNotBusySpin(t *testing.T) {
	s := memstreamer.New()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	const consumers = 8
	for i := 0; i < consumers; i++ {
		name := "consumer-" + string(rune('a'+i))
		rec, err := s.NewReceiver(ctx, testTopic, name)
		if err != nil {
			t.Fatalf("NewReceiver: %v", err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, _, err := rec.Recv(ctx)
				if err != nil {
					return
				}
			}
		}()
	}

	// Let consumers settle into cond.Wait.
	time.Sleep(100 * time.Millisecond)

	before := runtime.NumGoroutine()
	time.Sleep(200 * time.Millisecond)
	after := runtime.NumGoroutine()

	// Each Recv call internally spawns a ctx-watcher goroutine that lives
	// for the duration of the Recv call. With no Sends arriving the
	// receivers stay parked and goroutine count should be stable.
	if after > before+2 { // +2 wiggle for the test runtime itself
		t.Errorf("goroutine count grew while idle: before=%d after=%d", before, after)
	}

	// Verify receivers are parked, not busy-spinning. Dump all goroutine
	// stacks and assert that there are at least `consumers` goroutines
	// suspended on memstreamer.(*Stream).Recv via sync.Cond.Wait /
	// semaphore wait. Under the old busy-loop the Recv goroutines would
	// be in a runnable for-loop and would NOT show up as waiting in a
	// sync primitive inside Recv.
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	stacks := string(buf[:n])
	parked := strings.Count(stacks, "memstreamer.(*Stream).Recv")
	// Each Recv goroutine appears once; we expect at least `consumers`
	// stacks pointing into Recv, all in a wait state (sync.runtime_*).
	if parked < consumers {
		t.Errorf("expected %d goroutines parked in Recv, found %d in stacks", consumers, parked)
	}
	if !strings.Contains(stacks, "sync.runtime_notifyListWait") &&
		!strings.Contains(stacks, "sync.(*Cond).Wait") {
		t.Errorf("no goroutines parked on sync.Cond.Wait — Recv may be busy-spinning. Stacks:\n%s", stacks)
	}
}

// TestRecv_WakesOnSend asserts that a parked Recv returns promptly after a
// Send arrives. With the old busy-loop implementation the test still passed
// (because the loop polled the log), but it would also burn CPU for the
// 50ms park-window. With cond.Broadcast the wake is event-driven.
func TestRecv_WakesOnSend(t *testing.T) {
	s := memstreamer.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec, err := s.NewReceiver(ctx, testTopic, "wake-on-send")
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	t.Cleanup(func() { _ = rec.Close() })

	got := make(chan *workflow.Event, 1)
	go func() {
		e, ack, err := rec.Recv(ctx)
		if err != nil {
			return
		}
		_ = ack()
		got <- e
	}()

	// Give the Recv goroutine time to enter cond.Wait.
	time.Sleep(50 * time.Millisecond)

	send, err := s.NewSender(ctx, testTopic)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	t.Cleanup(func() { _ = send.Close() })

	sendAt := time.Now()
	if err := send.Send(ctx, "fid-1", 42, map[workflow.Header]string{
		workflow.HeaderTopic: testTopic,
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case e := <-got:
		if elapsed := time.Since(sendAt); elapsed > 100*time.Millisecond {
			t.Errorf("Recv took %v to wake after Send (want <100ms)", elapsed)
		}
		if e.ForeignID != "fid-1" {
			t.Errorf("ForeignID: want fid-1, got %q", e.ForeignID)
		}
	case <-time.After(time.Second):
		t.Fatalf("Recv did not unblock within 1s of Send")
	}
}

// TestRecv_WakesOnCtxCancel verifies that ctx cancellation wakes a parked
// Recv promptly and that the ctx-watcher goroutine does not leak per call.
func TestRecv_WakesOnCtxCancel(t *testing.T) {
	s := memstreamer.New()
	ctx, cancel := context.WithCancel(context.Background())

	rec, err := s.NewReceiver(ctx, testTopic, "wake-on-cancel")
	if err != nil {
		t.Fatalf("NewReceiver: %v", err)
	}
	t.Cleanup(func() { _ = rec.Close() })

	before := runtime.NumGoroutine()

	done := make(chan error, 1)
	go func() {
		_, _, err := rec.Recv(ctx)
		done <- err
	}()

	// Let it park.
	time.Sleep(20 * time.Millisecond)

	cancelAt := time.Now()
	cancel()

	select {
	case err := <-done:
		if elapsed := time.Since(cancelAt); elapsed > 100*time.Millisecond {
			t.Errorf("Recv took %v to wake after cancel (want <100ms)", elapsed)
		}
		if err == nil {
			t.Errorf("Recv should return ctx.Err(), got nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Recv did not unblock on ctx cancel within 500ms")
	}

	// Give the watcher goroutine a moment to exit after Recv returns.
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before {
		t.Errorf("goroutine leak after ctx cancel: before=%d after=%d", before, after)
	}
}
