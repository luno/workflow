package memstreamer_test

import (
	"testing"

	"github.com/andrewwormald/workflow/adapters/memstreamer"
	adapter "github.com/andrewwormald/workflow/adapters/testing"
)

func TestStreamer(t *testing.T) {
	constructor := memstreamer.New()
	adapter.TestStreamer(t, constructor)
}
