package errorcounter

import (
	"strings"
	"sync"
)

type ErrorCounter interface {
	Add(err error, labels ...string)
	Count(err error, labels ...string) int
	Clear(err error, labels ...string)
}

func New() ErrorCounter {
	return &counter{
		store: make(map[string]int),
	}
}

type counter struct {
	mu    sync.Mutex
	store map[string]int
}

func (c *counter) Add(err error, labels ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	errMsg := err.Error()
	errMsg += strings.Join(labels, "-")
	c.store[errMsg] += 1
}

func (c *counter) Count(err error, labels ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	errMsg := err.Error()
	errMsg += strings.Join(labels, "-")
	return c.store[errMsg]
}

func (c *counter) Clear(err error, labels ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	errMsg := err.Error()
	errMsg += strings.Join(labels, "-")
	c.store[errMsg] = 0
	return
}
