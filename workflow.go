package workflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/errorcounter"
	"github.com/luno/workflow/internal/graph"
	"github.com/luno/workflow/internal/metrics"
)

type API[Type any, Status StatusType] interface {
	// Name returns the name of the implemented workflow.
	Name() string

	// Trigger will kickstart a workflow Run for the provided foreignID starting from the default entrypoint to
	// the workflow which is the first "from" status added via the builder
	// (e.g. builder.AddStep(FromStatus, func{}, ToStatus). There is no limitation as to where you start the workflow
	// from and can do so via the WithStartAt trigger option. WithInitialValue should be used when you need data to be
	// present in the workflow Run before it starts. This can be used to reduce the need for duplicating reads.
	//
	// foreignID should not be random and should be deterministic for the thing that you are running the workflow for.
	// This especially helps when connecting other workflows as the foreignID is the only way to connect the streams. The
	// same goes for Callback as you will need the foreignID to connect the callback back to the workflow instance that
	// was run.
	Trigger(
		ctx context.Context,
		foreignID string,
		opts ...TriggerOption[Type, Status],
	) (runID string, err error)

	// Schedule takes a cron spec and will call Trigger at the specified intervals. Schedule is a blocking call and all
	// schedule errors will be retried indefinitely. The same options are available for Schedule as they are
	// for Trigger.
	Schedule(foreignID string, spec string, opts ...ScheduleOption[Type, Status]) error

	// Await is a blocking call that returns the typed Run when the workflow of the specified run ID reaches the
	// specified status.
	Await(ctx context.Context, foreignID, runID string, status Status, opts ...AwaitOption) (*Run[Type, Status], error)

	// Callback can be used if Builder.AddCallback has been defined for the provided status. The data in the reader
	// will be passed to the CallbackFunc that you specify and so the serialisation and deserialisation is in the
	// hands of the user.
	Callback(ctx context.Context, foreignID string, status Status, payload io.Reader) error

	// Run must be called in order to start up all the background consumers / consumers required to run the workflow. Run
	// only needs to be called once. Any subsequent calls to run are safe and are noop.
	Run(ctx context.Context)

	// Stop tells the workflow to shut down gracefully.
	Stop()
}

type Workflow[Type any, Status StatusType] struct {
	name      string
	ctx       context.Context
	cancel    context.CancelFunc
	clock     clock.Clock
	calledRun bool
	once      sync.Once
	logger    *logger

	eventStreamer EventStreamer
	recordStore   RecordStore
	timeoutStore  TimeoutStore
	scheduler     RoleScheduler

	consumers        map[Status]consumerConfig[Type, Status]
	callback         map[Status][]callback[Type, Status]
	timeouts         map[Status]timeouts[Type, Status]
	connectorConfigs []*connectorConfig[Type, Status]

	defaultOpts         options
	outboxConfig        outboxConfig
	pausedRecordsRetry  pausedRecordsRetry
	customDelete        customDelete
	runStateChangeHooks map[RunState]RunStateChangeHookFunc[Type, Status]

	internalStateMu sync.Mutex
	// internalState holds the State of all expected consumers and timeout go routines using their role names
	// as the key.
	internalState map[string]State
	// launching tracks the number of goroutines initiated but not yet running.
	// There's a non-deterministic delay between spawning a goroutine (`go myFunc()`)
	// and its addition to workflow's internalState. To ensure Run returns only after
	// all processes are recorded in internalState, launching provides a way to track
	// and block until this transition is complete.
	launching sync.WaitGroup

	statusGraph *graph.Graph
	// errorCounter keeps a central in-mem state of errors from consumers and timeouts in order to implement
	// PauseAfterErrCount. The tracking of errors is done in a way where errors need to be unique per process
	// (consumer / timeout).
	errorCounter errorcounter.ErrorCounter
}

func (w *Workflow[Type, Status]) Name() string {
	return w.name
}

func (w *Workflow[Type, Status]) Run(ctx context.Context) {
	// Ensure that the background consumers are only initialized once
	w.once.Do(func() {
		ctx, cancel := context.WithCancel(ctx)
		w.ctx = ctx
		w.cancel = cancel
		w.calledRun = true

		// Start the outbox consumer
		track(w, func() {
			outboxConsumer(w, w.outboxConfig)
		})

		// Start the state step consumers
		for currentStatus, config := range w.consumers {
			parallelCount := w.defaultOpts.parallelCount
			if config.parallelCount != 0 {
				parallelCount = config.parallelCount
			}

			if parallelCount < 2 {
				// Launch all consumers in runners
				track(w, func() {
					consumeStepEvents(w, currentStatus, config, 1, 1)
				})
			} else {
				// Run as sharded parallel consumers
				for i := 1; i <= parallelCount; i++ {
					track(w, func() {
						consumeStepEvents(w, currentStatus, config, i, parallelCount)
					})
				}
			}
		}

		// Only start timeout consumers if the timeout store is provided. This allows for the timeout store to
		// be optional for workflows where the timeout feature is not needed.
		if w.timeoutStore != nil {
			for status, timeouts := range w.timeouts {
				track(w, func() {
					timeoutPoller(w, status, timeouts)
				})
				track(w, func() {
					timeoutAutoInserterConsumer(w, status, timeouts)
				})
			}
		}

		// Start the connected stream consumers
		for _, config := range w.connectorConfigs {
			parallelCount := w.defaultOpts.parallelCount
			if config.parallelCount != 0 {
				parallelCount = config.parallelCount
			}

			if parallelCount < 2 {
				// Launch all consumers in runners
				track(w, func() {
					connectorConsumer(w, config, 1, 1)
				})
			} else {
				// Run as sharded parallel consumers
				for i := 1; i <= config.parallelCount; i++ {
					track(w, func() {
						connectorConsumer(w, config, i, config.parallelCount)
					})
				}
			}
		}

		// Launch all the run state change hooks that consume run state changes and respond according to the user's
		// configuration.
		for state, hook := range w.runStateChangeHooks {
			track(w, func() {
				runStateChangeHookConsumer(w, state, hook)
			})
		}

		// Launch the delete consumer which will manage all data deletion requests.
		track(w, func() {
			deleteConsumer(w)
		})

		// Only start the paused record retry consumer if enabled.
		if w.pausedRecordsRetry.enabled {
			track(w, func() {
				pausedRecordsRetryConsumer(w)
			})
		}
	})

	w.launching.Wait()
}

// track starts a new goroutine to execute the provided function and ensures
// it is tracked using launching.
func track[Type any, Status StatusType](w *Workflow[Type, Status], fn func()) {
	w.launching.Add(1)
	go fn()
}

// run is a standardise way of running blocking calls with a built-in retry mechanism.
func (w *Workflow[Type, Status]) run(
	role string,
	processName string,
	process func(ctx context.Context) error,
	errBackOff time.Duration,
) {
	w.updateState(processName, StateIdle)
	defer w.updateState(processName, StateShutdown)
	// Mark that another go routine has launched and been added to internal state
	w.launching.Done()

	for {
		err := runOnce(
			w.ctx,
			w.Name(),
			role,
			processName,
			w.updateState,
			w.scheduler.Await,
			process,
			w.logger,
			w.clock,
			errBackOff,
		)
		if err != nil {
			w.logger.Debug(w.ctx, "shutting down process", map[string]string{
				"role":         role,
				"process_name": processName,
			})

			return
		}
	}
}

type (
	updateStateFn func(processName string, s State)
	awaitRoleFn   func(ctx context.Context, role string) (context.Context, context.CancelFunc, error)
)

func runOnce(
	ctx context.Context,
	workflowName string,
	role string,
	processName string,
	updateState updateStateFn,
	awaitRole awaitRoleFn,
	process func(ctx context.Context) error,
	logger *logger,
	clock clock.Clock,
	errBackOff time.Duration,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	updateState(processName, StateIdle)

	ctx, cancel, err := awaitRole(ctx, role)
	if errors.Is(err, context.Canceled) {
		// Exit cleanly if error returned is cancellation of context
		return err
	} else if err != nil {
		logger.Error(ctx, fmt.Errorf("run error [role=%s], [process=%s]: %v", role, processName, err))

		// Return nil to try again
		return nil
	}
	defer cancel()

	updateState(processName, StateRunning)

	err = process(ctx)
	if errors.Is(err, context.Canceled) {
		// Context can be cancelled by the role scheduler and thus return nil to attempt to gain the role again
		// and if the parent context was cancelled then that will exit safely.
		return nil
	} else if err != nil {
		logger.Error(ctx, fmt.Errorf("run error [role=%s], [process=%s]: %v", role, processName, err))
		metrics.ProcessErrors.WithLabelValues(workflowName, processName).Inc()

		timer := clock.NewTimer(errBackOff)
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C():
			// Return nil to try again
			return nil
		}
	}

	return nil
}

// Stop cancels the context provided to all the background processes that the workflow launched and waits for all of
// them to shut down gracefully.
func (w *Workflow[Type, Status]) Stop() {
	if w.cancel == nil {
		return
	}

	// Cancel the parent context of the workflow to gracefully shutdown.
	w.cancel()

	for {
		var runningProcesses int
		for _, state := range w.States() {
			switch state {
			case StateUnknown, StateShutdown:
				continue
			default:
				runningProcesses++
			}
		}

		// Once all processes have exited then return
		if runningProcesses == 0 {
			return
		}
	}
}
