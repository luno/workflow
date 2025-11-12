package workflow_test

// Note: Tests for SaveAndRepeat transition validation and status maintenance
// are covered by the integration tests in workflow_test.go since they depend
// on internal implementation details like record storage format and internal
// function behavior that are not exposed through the public API.
//
// The key behaviors tested in workflow_test.go include:
// - SaveAndRepeat skips transition validation
// - SaveAndRepeat maintains the current status
// - SaveAndRepeat works with various edge cases (context cancellation, timeout, etc.)