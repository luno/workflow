package workflow

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/jettison/log"
	"k8s.io/utils/clock"

	"github.com/luno/workflow/internal/metrics"
)

type API[Type any, Status StatusType] interface {
	// Trigger will kickstart a workflow for the provided foreignID starting from the provided starting status. There
	// is no limitation as to where you start the workflow from. For workflows that have data preceding the initial
	// trigger that needs to be used in the workflow, using WithInitialValue will allow you to provide pre-populated
	// fields of Type that can be accessed by the consumers.
	//
	// foreignID should not be random and should be deterministic for the thing that you are running the workflow for.
	// This especially helps when connecting other workflows as the foreignID is the only way to connect the streams. The
	// same goes for Callback as you will need the foreignID to connect the callback back to the workflow instance that
	// was run.
	Trigger(ctx context.Context, foreignID string, startingStatus Status, opts ...TriggerOption[Type, Status]) (runID string, err error)

	// Schedule takes a cron spec and will call Trigger at the specified intervals. Schedule is a blocking call and will
	// return ErrWorkflowNotRunning or ErrStatusProvidedNotConfigured to indicate that it cannot begin to schedule. All
	// schedule errors will be retried indefinitely. The same options are available for Schedule as they are
	// for Trigger.
	Schedule(ctx context.Context, foreignID string, startingStatus Status, spec string, opts ...ScheduleOption[Type, Status]) error

	// Await is a blocking call that returns the typed Record when the workflow of the specified run ID reaches the
	// specified status.
	Await(ctx context.Context, foreignID, runID string, status Status, opts ...AwaitOption) (*Record[Type, Status], error)

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
	Name string

	ctx    context.Context
	cancel context.CancelFunc

	clock                   clock.Clock
	defaultPollingFrequency time.Duration
	defaultErrBackOff       time.Duration
	defaultLagAlert         time.Duration

	calledRun bool
	once      sync.Once

	eventStreamer EventStreamer
	recordStore   RecordStore
	timeoutStore  TimeoutStore
	scheduler     RoleScheduler

	consumers        map[Status][]consumerConfig[Type, Status]
	callback         map[Status][]callback[Type, Status]
	timeouts         map[Status]timeouts[Type, Status]
	connectorConfigs []connectorConfig[Type, Status]
	outboxConfig     outboxConfig

	internalStateMu sync.Mutex
	// internalState holds the State of all expected consumers and timeout  go routines using their role names
	// as the key.
	internalState map[string]State

	graph      map[int][]int
	graphOrder []int

	endPoints     map[Status]bool
	validStatuses map[Status]bool

	debugMode bool
}

func (w *Workflow[Type, Status]) Run(ctx context.Context) {
	// Ensure that the background consumers are only initialized once
	w.once.Do(func() {
		ctx, cancel := context.WithCancel(ctx)
		w.ctx = ctx
		w.cancel = cancel
		w.calledRun = true

		// Start the outbox consumers
		if w.outboxConfig.parallelCount < 2 {
			// Launch all consumers in runners
			go outboxConsumer(w, w.outboxConfig, 1, 1)
		} else {
			// Run as sharded parallel consumers
			for i := 1; i <= w.outboxConfig.parallelCount; i++ {
				go outboxConsumer(w, w.outboxConfig, i, w.outboxConfig.parallelCount)
			}
		}

		// Start the state transition consumers
		for currentStatus, consumers := range w.consumers {
			for _, p := range consumers {
				if p.parallelCount < 2 {
					// Launch all consumers in runners
					go consumer(w, currentStatus, p, 1, 1)
				} else {
					// Run as sharded parallel consumers
					for i := 1; i <= p.parallelCount; i++ {
						go consumer(w, currentStatus, p, i, p.parallelCount)
					}
				}
			}
		}

		// Start the timeout poller and inserter consumer
		for status, timeouts := range w.timeouts {
			go timeoutPoller(w, status, timeouts)
			go timeoutAutoInserterConsumer(w, status, timeouts)
		}

		// Start the connected stream consumers
		for _, config := range w.connectorConfigs {
			if config.parallelCount < 2 {
				// Launch all consumers in runners
				go connectorConsumer(w, &config, 1, 1)
			} else {
				// Run as sharded parallel consumers
				for i := 1; i <= config.parallelCount; i++ {
					go connectorConsumer(w, &config, i, config.parallelCount)
				}
			}
		}
	})
}

// run is a standardise way of running blocking calls forever with retry such as consumers that need to adhere to role scheduling
func (w *Workflow[Type, Status]) run(role, processName string, process func(ctx context.Context) error, errBackOff time.Duration) {
	w.updateState(processName, StateIdle)
	defer w.updateState(processName, StateShutdown)

	for {
		err := runOnce(w, role, processName, process, errBackOff)
		if err != nil {
			if w.debugMode {
				log.Info(w.ctx, "shutting down process", j.MKV{
					"role":         role,
					"process_name": processName,
				})
			}
			return
		}
	}
}

func runOnce[Type any, Status StatusType](w *Workflow[Type, Status], role, processName string, process func(ctx context.Context) error, errBackOff time.Duration) error {
	w.updateState(processName, StateIdle)

	ctx, cancel, err := w.scheduler.Await(w.ctx, role)
	if errors.IsAny(err, context.Canceled) {
		// Exit cleanly if error returned is cancellation of context
		return err
	} else if err != nil {
		log.Error(ctx, errors.Wrap(err, "error awaiting role", j.MKV{
			"role":         role,
			"process_name": processName,
		}))

		// Return nil to try again
		return nil
	}
	defer cancel()

	w.updateState(processName, StateRunning)

	err = process(ctx)
	if errors.Is(err, context.Canceled) {
		// Context can be cancelled by the role scheduler and thus return nil to attempt to gain the role again
		// and if the parent context was cancelled then that will exit safely.
		return nil
	} else if err != nil {
		log.Error(ctx, errors.Wrap(err, "process error", j.MKV{
			"role": role,
		}))
		metrics.ProcessErrors.WithLabelValues(w.Name, processName).Inc()

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(errBackOff):
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

func safeUpdate(ctx context.Context, store RecordStore, graph map[int][]int, currentStatus int, next *WireRecord) error {
	latest, err := store.Lookup(ctx, next.ID)
	if err != nil {
		return err
	}

	// Ensure that the record is still has the intended status. If not then another consumer will be processing this
	// record.
	if latest.Status != currentStatus {
		return nil
	}

	// Lookup all available transitions from the current status
	nodes, ok := graph[currentStatus]
	if !ok {
		return errors.New("current status not predefined", j.MKV{
			"current_status": currentStatus,
		})
	}

	var found bool
	// Attempt to find the next status amongst the list of valid transitions
	for _, node := range nodes {
		if node == next.Status {
			found = true
			break
		}
	}

	// If no valid transition matches that of the next status then error.
	if !found {
		return errors.New("invalid transition attempted", j.MKV{
			"current_status": currentStatus,
			"next_status":    next.Status,
		})
	}

	return storeAndEmit(ctx, store, next)
}

func storeAndEmit(ctx context.Context, store RecordStore, wr *WireRecord) error {
	return store.Store(ctx, wr, func(recordID int64) (OutboxEventData, error) {
		// Record ID would not have been set if it is a new record. Assign the recordID that the Store provides
		wr.ID = recordID
		return WireRecordToOutboxEventData(*wr)
	})
}
