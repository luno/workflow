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

func (c *Counter) Add(label string, extras ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(label, extras)
	c.store[key] += 1
	return c.store[key]
}

func (c *Counter) Count(label string, extras ...string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(label, extras)
	return c.store[key]
}

func (c *Counter) Clear(label string, extras ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := makeKey(label, extras)
	delete(c.store, key)
}

func makeKey(label string, extras []string) string {
	if len(extras) == 0 {
		return label
	}
	return label + "-" + strings.Join(extras, "-")
}
