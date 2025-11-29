package errorcounter

import (
	"strings"
	"sync"
)

// New creates and returns a Counter with an initialised internal store ready for use.
func New() *Counter {
	return &Counter{
		store: make(map[string]int),
	}
}

type Counter struct {
	mu    sync.Mutex
	store map[string]int
}

func (c *Counter) Add(err error, labels ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	errMsg := err.Error()
	errMsg += strings.Join(labels, "-")
	c.store[errMsg] += 1
	return c.store[errMsg]
}

func (c *Counter) Count(err error, labels ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	errMsg := err.Error()
	errMsg += strings.Join(labels, "-")
	return c.store[errMsg]
}

func (c *Counter) Clear(err error, labels ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	errMsg := err.Error()
	errMsg += strings.Join(labels, "-")
	c.store[errMsg] = 0
	return
}