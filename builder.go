package workflow

import (
	"context"
	"path"
	"time"

	"k8s.io/utils/clock"
)

const (
	defaultPollingFrequency = 500 * time.Millisecond
	defaultErrBackOff       = 1 * time.Second
	defaultLagAlert         = 30 * time.Minute

	defaultOutboxLagAlert         = time.Minute
	defaultOutboxPollingFrequency = 250 * time.Millisecond
	defaultOutboxErrBackOff       = 500 * time.Millisecond
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
		destinationStatus: to,
		consumer:          c,
	}

	var so stepOptions
	for _, opt := range opts {
		opt(&so)
	}

	if so.parallelCount > 0 {
		p.parallelCount = so.parallelCount
	}

	p.pollingFrequency = b.workflow.defaultPollingFrequency
	if so.pollingFrequency.Nanoseconds() != 0 {
		p.pollingFrequency = so.pollingFrequency
	}

	p.errBackOff = b.workflow.defaultErrBackOff
	if so.errBackOff.Nanoseconds() != 0 {
		p.errBackOff = so.errBackOff
	}

	if so.lag.Nanoseconds() != 0 {
		p.lag = so.lag
	}

	p.lagAlert = b.workflow.defaultLagAlert

	// If lag is specified then offset the lag alert by the default amount. Custom lag alert values, if set, will
	// still take priority.
	if p.lag.Nanoseconds() != 0 {
		p.lagAlert = b.workflow.defaultLagAlert + p.lag
	}

	// If a customer lag alert is provided then override the current defaults.
	if so.lagAlert.Nanoseconds() != 0 {
		p.lagAlert = so.lagAlert
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

func (b *Builder[Type, Status]) AddConnector(name string, c Consumer, cf ConnectorFunc[Type, Status], opts ...ConnectorOption) {
	var connectorOptions connectorOptions
	for _, opt := range opts {
		opt(&connectorOptions)
	}

	if connectorOptions.errBackOff.Nanoseconds() == 0 {
		connectorOptions.errBackOff = defaultErrBackOff
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
	b.workflow.eventStreamer = eventStreamer
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

	b.workflow.outboxConfig = defaultOutboxConfig()
	if bo.outboxConfig != nil {
		b.workflow.outboxConfig = *bo.outboxConfig
	}

	if b.workflow.defaultPollingFrequency.Milliseconds() == 0 {
		b.workflow.defaultPollingFrequency = time.Second
	}

	b.workflow.endPoints = b.determineEndPoints(b.workflow.graph)
	b.workflow.debugMode = bo.debugMode

	return b.workflow
}

type buildOptions struct {
	clock        clock.Clock
	debugMode    bool
	outboxConfig *outboxConfig
}

type BuildOption func(w *buildOptions)

func WithClock(c clock.Clock) BuildOption {
	return func(bo *buildOptions) {
		bo.clock = c
	}
}

func WithOutboxConfig(opts ...OutboxOption) BuildOption {
	return func(bo *buildOptions) {
		config := defaultOutboxConfig()

		for _, opt := range opts {
			opt(&config)
		}

		bo.outboxConfig = &config
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
			key := path.Join(s.String(), p.destinationStatus.String())
			if dedupe[key] {
				continue
			}

			graph[s] = append(graph[s], p.destinationStatus)
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
