package stack

import (
	"runtime"
	"strings"
)

// Trace returns a stack trace of the external caller. Depth refers to the depth inside workflow that Trace is being
// called from. So if the exported function in Workflow that is being called is 2 calls up then depth should be 2.
func Trace(depth int) string {
	// Capture at most 4KB of the stack trace
	stack := make([]byte, 4096)
	n := runtime.Stack(stack, false)

	// Convert the stack trace to a string and split it by newlines.
	trace := string(stack[:n])
	lines := strings.Split(trace, "\n")

	var (
		stackTrace     string
		remainingDepth = depth + 1 // Add one to remove the stack entry of "/internal/stack/stack.go"
	)
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

		if remainingDepth != 0 {
			remainingDepth--
			continue
		}

		stackTrace += line + "\n"
	}

	return stackTrace
}
