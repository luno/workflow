package kafkastreamer_test

import (
	"testing"

	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/luno/workflow/adapters/kafkastreamer"
)

func TestStreamer(t *testing.T) {
	constructor := kafkastreamer.New([]string{"localhost:9092"})
	adaptertest.RunEventStreamerTest(t, constructor)
}
