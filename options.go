package workflow

import "time"

// options provides a common option configuration structure for steps, callbacks, and timeouts.
type options struct {
	parallelCount    int
	pollingFrequency time.Duration
	errBackOff       time.Duration
	lag              time.Duration

	lagAlert          time.Duration
	customLagAlertSet bool

	// pauseAfterErrCount defines the number of errors before moving the record to RunStatePaused. Value of 0 will be
	// treated as it not being configured and the user will retry forever as is default behaviour.
	pauseAfterErrCount int
}

func defaultOptions() options {
	return options{
		pollingFrequency: defaultPollingFrequency,
		errBackOff:       defaultErrBackOff,
		lagAlert:         defaultLagAlert,
	}
}

type Option func(so *options)

// ParallelCount defines the number of instances of the workflow process. The processes are shareded consistently
// and will be provided a name such as "consumer-1-of-5" to show the instance number and the total number of instances
// that the process is a part of.
func ParallelCount(instances int) Option {
	return func(opt *options) {
		opt.parallelCount = instances
	}
}

// PollingFrequency defines the time duration of which the workflow process will poll for changes.
func PollingFrequency(d time.Duration) Option {
	return func(opt *options) {
		opt.pollingFrequency = d
	}
}

// ErrBackOff defines the time duration of the backoff of the workflow process when an error is encountered.
func ErrBackOff(d time.Duration) Option {
	return func(opt *options) {
		opt.errBackOff = d
	}
}

// LagAlert defines the time duration / threshold before the prometheus metric defined in /internal/metrics/metrics.go switches to
// true which means that the workflow consumer is struggling to consume events fast enough and might need to be converted
// to a parallel consumer.
func LagAlert(d time.Duration) Option {
	return func(opt *options) {
		opt.lagAlert = d
		opt.customLagAlertSet = true
	}
}

// ConsumeLag defines the age of the event that the consumer will consume. The workflow consumer will not consume events
// newer than the time specified and will wait to consume them.
func ConsumeLag(d time.Duration) Option {
	return func(opt *options) {
		opt.lag = d

		if !opt.customLagAlertSet {
			// Ensure that the lag alert is offset by the added lag.
			opt.lagAlert = defaultLagAlert + d
		}
	}
}

// PauseAfterErrCount defines the number of times an error can occur until the record is updated to RunStatePaused
// which is similar to a Dead Letter Queue in the sense that the record will no longer be processed and wont block
// workflow consumers and can be investigated and retried.
func PauseAfterErrCount(count int) Option {
	return func(opt *options) {
		opt.pauseAfterErrCount = count
	}
}
