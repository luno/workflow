package workflow

import (
	"context"
	"path"
	"time"

	"k8s.io/utils/clock"
)

const (
	defaultPollingFrequency = 500 * time.Millisecond
	defaultErrBackOff       = 5 * time.Second
	defaultLagAlert         = 30 * time.Minute
)

func NewBuilder[Type any, Status StatusType](name string) *Builder[Type, Status] {
	return &Builder[Type, Status]{
		workflow: &Workflow[Type, Status]{
			Name:                    name,
			clock:                   clock.RealClock{},
			defaultPollingFrequency: defaultPollingFrequency,
			defaultErrBackOff:       defaultErrBackOff,
			defaultLagAlert:         defaultLagAlert,
			consumers:               make(map[Status][]consumerConfig[Type, Status]),
			callback:                make(map[Status][]callback[Type, Status]),
			timeouts:                make(map[Status]timeouts[Type, Status]),
			graph:                   make(map[int][]int),
			validStatuses:           make(map[Status]bool),
			internalState:           make(map[string]State),
		},
	}
}

type Builder[Type any, Status StatusType] struct {
	workflow *Workflow[Type, Status]
}

func (b *Builder[Type, Status]) AddStep(from Status, c ConsumerFunc[Type, Status], to Status, opts ...StepOption) {
	p := consumerConfig[Type, Status]{
		DestinationStatus: to,
		Consumer:          c,
	}

	var so stepOptions
	for _, opt := range opts {
		opt(&so)
	}

	if so.parallelCount > 0 {
		p.ParallelCount = so.parallelCount
	}

	p.PollingFrequency = b.workflow.defaultPollingFrequency
	if so.pollingFrequency.Nanoseconds() != 0 {
		p.PollingFrequency = so.pollingFrequency
	}

	p.ErrBackOff = b.workflow.defaultErrBackOff
	if so.errBackOff.Nanoseconds() != 0 {
		p.ErrBackOff = so.errBackOff
	}

	if so.lag.Nanoseconds() != 0 {
		p.Lag = so.lag
	}

	p.LagAlert = b.workflow.defaultLagAlert

	// If lag is specified then offset the lag alert by the default amount. Custom lag alert values, if set, will
	// still take priority.
	if p.Lag.Nanoseconds() != 0 {
		p.LagAlert = b.workflow.defaultLagAlert + p.Lag
	}

	// If a customer lag alert is provided then override the current defaults.
	if so.lagAlert.Nanoseconds() != 0 {
		p.LagAlert = so.lagAlert
	}

	b.workflow.graph[int(from)] = append(b.workflow.graph[int(from)], int(to))
	b.workflow.graphOrder = append(b.workflow.graphOrder, int(from))
	b.workflow.validStatuses[from] = true
	b.workflow.validStatuses[to] = true
	b.workflow.consumers[from] = append(b.workflow.consumers[from], p)
}

type stepOptions struct {
	parallelCount    int
	pollingFrequency time.Duration
	errBackOff       time.Duration
	lag              time.Duration
	lagAlert         time.Duration
}

type StepOption func(so *stepOptions)

func WithParallelCount(instances int) StepOption {
	return func(so *stepOptions) {
		so.parallelCount = instances
	}
}

func WithStepPollingFrequency(d time.Duration) StepOption {
	return func(so *stepOptions) {
		so.pollingFrequency = d
	}
}

func WithStepErrBackOff(d time.Duration) StepOption {
	return func(so *stepOptions) {
		so.errBackOff = d
	}
}

func WithStepLagAlert(d time.Duration) StepOption {
	return func(so *stepOptions) {
		so.lagAlert = d
	}
}

func WithStepConsumerLag(d time.Duration) StepOption {
	return func(so *stepOptions) {
		so.lag = d
	}
}

func (b *Builder[Type, Status]) AddCallback(from Status, fn CallbackFunc[Type, Status], to Status) {
	c := callback[Type, Status]{
		DestinationStatus: to,
		CallbackFunc:      fn,
	}

	b.workflow.graph[int(from)] = append(b.workflow.graph[int(from)], int(to))
	b.workflow.graphOrder = append(b.workflow.graphOrder, int(from))
	b.workflow.validStatuses[from] = true
	b.workflow.validStatuses[to] = true
	b.workflow.callback[from] = append(b.workflow.callback[from], c)
}

type timeoutOptions struct {
	pollingFrequency time.Duration
	errBackOff       time.Duration
	lagAlert         time.Duration
}

type TimeoutOption func(so *timeoutOptions)

func WithTimeoutPollingFrequency(d time.Duration) TimeoutOption {
	return func(to *timeoutOptions) {
		to.pollingFrequency = d
	}
}

func WithTimeoutErrBackOff(d time.Duration) TimeoutOption {
	return func(to *timeoutOptions) {
		to.errBackOff = d
	}
}

func WithTimeoutLagAlert(d time.Duration) TimeoutOption {
	return func(to *timeoutOptions) {
		to.lagAlert = d
	}
}

func (b *Builder[Type, Status]) AddTimeout(from Status, timer TimerFunc[Type, Status], tf TimeoutFunc[Type, Status], to Status, opts ...TimeoutOption) {
	timeouts := b.workflow.timeouts[from]

	t := timeout[Type, Status]{
		DestinationStatus: to,
		TimerFunc:         timer,
		TimeoutFunc:       tf,
	}

	var topt timeoutOptions
	for _, opt := range opts {
		opt(&topt)
	}

	timeouts.PollingFrequency = b.workflow.defaultPollingFrequency
	if topt.pollingFrequency.Nanoseconds() != 0 {
		timeouts.PollingFrequency = topt.pollingFrequency
	}

	timeouts.ErrBackOff = b.workflow.defaultErrBackOff
	if topt.errBackOff.Nanoseconds() != 0 {
		timeouts.ErrBackOff = topt.errBackOff
	}

	timeouts.LagAlert = b.workflow.defaultLagAlert
	if topt.lagAlert.Nanoseconds() != 0 {
		timeouts.LagAlert = topt.lagAlert
	}

	timeouts.Transitions = append(timeouts.Transitions, t)

	b.workflow.graph[int(from)] = append(b.workflow.graph[int(from)], int(to))
	b.workflow.graphOrder = append(b.workflow.graphOrder, int(from))
	b.workflow.validStatuses[from] = true
	b.workflow.validStatuses[to] = true
	b.workflow.timeouts[from] = timeouts
}

func (b *Builder[Type, Status]) AddWorkflowConnector(cd WorkflowConnectionDetails, filter ConnectorFilter, from Status, consumer ConnectorConsumerFunc[Type, Status], to Status, opts ...StepOption) {
	var stepOptions stepOptions
	for _, opt := range opts {
		opt(&stepOptions)
	}

	b.workflow.validStatuses[to] = true
	b.workflow.workflowConnectorConfigs = append(b.workflow.workflowConnectorConfigs, workflowConnectorConfig[Type, Status]{
		workflowName:     cd.WorkflowName,
		status:           cd.Status,
		stream:           cd.Stream,
		filter:           filter,
		from:             from,
		consumer:         consumer,
		to:               to,
		pollingFrequency: stepOptions.pollingFrequency,
		errBackOff:       stepOptions.errBackOff,
		parallelCount:    stepOptions.parallelCount,
	})
}

func (b *Builder[Type, Status]) AddConnector(name string, c Consumer, cf ConnectorFunc[Type, Status], opts ...ConnectorOption) {
	var connectorOptions connectorOptions
	for _, opt := range opts {
		opt(&connectorOptions)
	}

	for _, config := range b.workflow.connectorConfigs {
		if config.name == name {
			panic("connector names need to be unique")
		}
	}

	b.workflow.connectorConfigs = append(b.workflow.connectorConfigs, connectorConfig[Type, Status]{
		name:          name,
		consumerFn:    c,
		connectorFn:   cf,
		errBackOff:    connectorOptions.errBackOff,
		parallelCount: connectorOptions.parallelCount,
	})
}

func (b *Builder[Type, Status]) Build(eventStreamer EventStreamer, recordStore RecordStore, timeoutStore TimeoutStore, roleScheduler RoleScheduler, opts ...BuildOption) *Workflow[Type, Status] {
	b.workflow.eventStreamerFn = eventStreamer
	b.workflow.recordStore = recordStore
	b.workflow.timeoutStore = timeoutStore
	b.workflow.scheduler = roleScheduler

	var bo buildOptions
	for _, opt := range opts {
		opt(&bo)
	}

	if bo.clock != nil {
		b.workflow.clock = bo.clock
	}

	if b.workflow.defaultPollingFrequency.Milliseconds() == 0 {
		b.workflow.defaultPollingFrequency = time.Second
	}

	b.workflow.endPoints = b.determineEndPoints(b.workflow.graph)
	b.workflow.debugMode = bo.debugMode

	return b.workflow
}

type buildOptions struct {
	clock     clock.Clock
	debugMode bool
}

type BuildOption func(w *buildOptions)

func WithClock(c clock.Clock) BuildOption {
	return func(bo *buildOptions) {
		bo.clock = c
	}
}

func WithDebugMode() BuildOption {
	return func(bo *buildOptions) {
		bo.debugMode = true
	}
}

func (b *Builder[Type, Status]) buildGraph() map[Status][]Status {
	graph := make(map[Status][]Status)
	dedupe := make(map[string]bool)
	for s, i := range b.workflow.consumers {
		for _, p := range i {
			key := path.Join(s.String(), p.DestinationStatus.String())
			if dedupe[key] {
				continue
			}

			graph[s] = append(graph[s], p.DestinationStatus)
			dedupe[key] = true
		}
	}

	for s, i := range b.workflow.callback {
		for _, c := range i {
			key := path.Join(s.String(), c.DestinationStatus.String())
			if dedupe[key] {
				continue
			}

			graph[s] = append(graph[s], c.DestinationStatus)
			dedupe[key] = true
		}
	}

	for s, t := range b.workflow.timeouts {
		for _, t := range t.Transitions {
			key := path.Join(s.String(), t.DestinationStatus.String())
			if dedupe[key] {
				continue
			}

			graph[s] = append(graph[s], t.DestinationStatus)
			dedupe[key] = true
		}
	}

	return graph
}

func (b *Builder[Type, Status]) determineEndPoints(graph map[int][]int) map[Status]bool {
	endpoints := make(map[Status]bool)
	for _, destinations := range graph {
		for _, destination := range destinations {
			_, ok := graph[destination]
			if !ok {
				// end points are nodes that do not have any of their own transitions to transition to.
				endpoints[Status(destination)] = true
			}
		}
	}

	return endpoints
}

func Not[Type any, Status StatusType](c ConsumerFunc[Type, Status]) ConsumerFunc[Type, Status] {
	return func(ctx context.Context, r *Record[Type, Status]) (bool, error) {
		pass, err := c(ctx, r)
		if err != nil {
			return false, err
		}

		return !pass, nil
	}
}

func DurationTimerFunc[Type any, Status StatusType](duration time.Duration) TimerFunc[Type, Status] {
	return func(ctx context.Context, r *Record[Type, Status], now time.Time) (time.Time, error) {
		return now.Add(duration), nil
	}
}

func TimeTimerFunc[Type any, Status StatusType](t time.Time) TimerFunc[Type, Status] {
	return func(ctx context.Context, r *Record[Type, Status], now time.Time) (time.Time, error) {
		return t, nil
	}
}
