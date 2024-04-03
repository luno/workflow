package workflow

import (
	"context"
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

func (b *Builder[Type, Status]) AddStep(from Status, c ConsumerFunc[Type, Status], allowedDestinations ...Status) *stepUpdater[Type, Status] {
	b.workflow.validStatuses[from] = true

	if _, ok := b.workflow.graph[int(from)]; !ok {
		b.workflow.graphOrder = append(b.workflow.graphOrder, int(from))
	}

	for _, to := range allowedDestinations {
		b.workflow.graph[int(from)] = append(b.workflow.graph[int(from)], int(to))
		b.workflow.validStatuses[to] = true
	}

	p := consumerConfig[Type, Status]{
		consumer:         c,
		pollingFrequency: b.workflow.defaultPollingFrequency,
		errBackOff:       b.workflow.defaultErrBackOff,
		lagAlert:         b.workflow.defaultLagAlert,
	}

	index := len(b.workflow.consumers[from])
	b.workflow.consumers[from] = append(b.workflow.consumers[from], p)

	return &stepUpdater[Type, Status]{
		from:     from,
		index:    index,
		workflow: b.workflow,
	}
}

type stepUpdater[Type any, Status StatusType] struct {
	from     Status
	index    int
	workflow *Workflow[Type, Status]
}

func (s *stepUpdater[Type, Status]) WithOptions(opts ...Option) {
	consumer := s.workflow.consumers[s.from][s.index]

	consumerOpts := options{
		parallelCount:    consumer.parallelCount,
		pollingFrequency: consumer.pollingFrequency,
		errBackOff:       consumer.errBackOff,
		lag:              consumer.lag,
		lagAlert:         consumer.lagAlert,
	}
	for _, opt := range opts {
		opt(&consumerOpts)
	}

	consumer.pollingFrequency = consumerOpts.pollingFrequency
	consumer.parallelCount = consumerOpts.parallelCount
	consumer.errBackOff = consumerOpts.errBackOff
	consumer.lag = consumerOpts.lag
	consumer.lagAlert = consumerOpts.lagAlert
	s.workflow.consumers[s.from][s.index] = consumer
}

func (b *Builder[Type, Status]) AddCallback(from Status, fn CallbackFunc[Type, Status], allowedDestinations ...Status) {
	c := callback[Type, Status]{
		CallbackFunc: fn,
	}

	if _, ok := b.workflow.graph[int(from)]; !ok {
		b.workflow.graphOrder = append(b.workflow.graphOrder, int(from))
	}

	for _, to := range allowedDestinations {
		b.workflow.graph[int(from)] = append(b.workflow.graph[int(from)], int(to))
		b.workflow.validStatuses[to] = true
	}

	b.workflow.validStatuses[from] = true
	b.workflow.callback[from] = append(b.workflow.callback[from], c)
}

func (b *Builder[Type, Status]) AddTimeout(from Status, timer TimerFunc[Type, Status], tf TimeoutFunc[Type, Status], allowedDestinations ...Status) *timeoutUpdater[Type, Status] {
	timeouts := b.workflow.timeouts[from]

	t := timeout[Type, Status]{
		TimerFunc:   timer,
		TimeoutFunc: tf,
	}

	if timeouts.pollingFrequency.Nanoseconds() == 0 {
		timeouts.pollingFrequency = b.workflow.defaultPollingFrequency
	}

	if timeouts.errBackOff.Nanoseconds() == 0 {
		timeouts.errBackOff = b.workflow.defaultErrBackOff
	}

	if timeouts.lagAlert.Nanoseconds() == 0 {
		timeouts.lagAlert = b.workflow.defaultLagAlert
	}

	b.workflow.validStatuses[from] = true

	if _, ok := b.workflow.graph[int(from)]; !ok {
		b.workflow.graphOrder = append(b.workflow.graphOrder, int(from))
	}

	for _, to := range allowedDestinations {
		b.workflow.graph[int(from)] = append(b.workflow.graph[int(from)], int(to))
		b.workflow.validStatuses[to] = true
	}

	timeouts.transitions = append(timeouts.transitions, t)
	b.workflow.timeouts[from] = timeouts

	return &timeoutUpdater[Type, Status]{
		from:     from,
		workflow: b.workflow,
	}
}

type timeoutUpdater[Type any, Status StatusType] struct {
	from     Status
	workflow *Workflow[Type, Status]
}

func (s *timeoutUpdater[Type, Status]) WithOptions(opts ...Option) {
	timeout := s.workflow.timeouts[s.from]

	timeoutOpts := options{
		pollingFrequency: timeout.pollingFrequency,
		errBackOff:       timeout.errBackOff,
		lagAlert:         timeout.lagAlert,
	}
	for _, opt := range opts {
		opt(&timeoutOpts)
	}

	if timeoutOpts.parallelCount != 0 {
		panic("Cannot configure parallel timeout")
	}

	if timeoutOpts.lag != 0 {
		panic("Cannot configure lag for timeout")
	}

	timeout.pollingFrequency = timeoutOpts.pollingFrequency
	timeout.errBackOff = timeoutOpts.errBackOff
	timeout.lagAlert = timeoutOpts.lagAlert
	s.workflow.timeouts[s.from] = timeout
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
