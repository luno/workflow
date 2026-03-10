package errorcounter

import (
	"strings"
	"sync"
)

func New() *Counter {
	return &Counter{
		store: make(map[string]int),
	}
}

type Counter struct {
	mu    sync.Mutex
	store map[string]int
}

func (c *Counter) Add(err error, label string, extras ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(label, extras)
	c.store[key] += 1
	return c.store[key]
}

func (c *Counter) Count(err error, label string, extras ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(label, extras)
	return c.store[key]
}

func (c *Counter) Clear(err error, label string, extras ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(label, extras)
	delete(c.store, key)
}

// makeKey builds a stable key from labels only. The error message is excluded
// because it often contains dynamic data (timestamps, IDs) which would create
// unique keys and prevent the PauseAfterErrCount threshold from ever being reached.
func makeKey(label string, extras []string) string {
	if len(extras) == 0 {
		return label
	}
	return label + "-" + strings.Join(extras, "-")
}
