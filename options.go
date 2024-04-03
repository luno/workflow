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
}

type Option func(so *options)

func ParallelCount(instances int) Option {
	return func(opt *options) {
		opt.parallelCount = instances
	}
}

func PollingFrequency(d time.Duration) Option {
	return func(opt *options) {
		opt.pollingFrequency = d
	}
}

func ErrBackOff(d time.Duration) Option {
	return func(opt *options) {
		opt.errBackOff = d
	}
}

func LagAlert(d time.Duration) Option {
	return func(opt *options) {
		opt.lagAlert = d
		opt.customLagAlertSet = true
	}
}

func ConsumeLag(d time.Duration) Option {
	return func(opt *options) {
		opt.lag = d

		if !opt.customLagAlertSet {
			// Ensure that the lag alert is offset by the added lag.
			opt.lagAlert = opt.lagAlert + d
		}
	}
}
