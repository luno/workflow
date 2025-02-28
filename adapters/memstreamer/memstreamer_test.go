package memstreamer_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/memstreamer"
)

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
