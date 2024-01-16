package memstreamer_test

import (
	"testing"

	"github.com/luno/workflow/adapters/memstreamer"
	adapter "github.com/luno/workflow/adapters/testing"
)

func TestStreamer(t *testing.T) {
	constructor := memstreamer.New()
	adapter.TestStreamer(t, constructor)
}
