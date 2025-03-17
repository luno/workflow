package stack

import (
	"runtime"
	"strings"
)

// Trace returns a stack trace of the external caller. Depth refers to the depth inside workflow that Trace is being
// called from.
func Trace() string {
	// Capture at most 4KB of the stack trace
	stack := make([]byte, 4096)
	n := runtime.Stack(stack, false)

	// Convert the stack trace to a string and split it by newlines.
	trace := string(stack[:n])
	lines := strings.Split(trace, "\n")

	var stackTrace string
	for _, line := range lines {
		if strings.Contains(line, "github.com/luno/workflow") {
			continue
		}

		if strings.Contains(line, "goroutine") {
			continue
		}

		if !strings.Contains(line, "/") {
			continue
		}

		stackTrace += line + "\n"
	}

	return stackTrace
}
