package memstreamer_test

import (
	"testing"

	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memstreamer"
)

func TestStreamer(t *testing.T) {
	constructor := memstreamer.New()
	adaptertest.RunEventStreamerTest(t, constructor)
}
