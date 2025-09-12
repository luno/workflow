package kafkastreamer

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"github.com/luno/workflow"
)

func New(brokers []string) *StreamConstructor {
	return &StreamConstructor{
		sharedConfig: newConfig(),
		brokers:      brokers,
	}
}

func newConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	return config
}

var _ workflow.EventStreamer = (*StreamConstructor)(nil)

type StreamConstructor struct {
	sharedConfig *sarama.Config
	brokers      []string
}

func (s StreamConstructor) NewSender(ctx context.Context, topic string) (workflow.EventSender, error) {
	producer, err := sarama.NewSyncProducer(s.brokers, s.sharedConfig)
	if err != nil {
		return nil, err
	}

	return &Sender{
		Topic:         topic,
		Writer:        producer,
		WriterTimeout: time.Second * 10,
	}, nil
}

type Sender struct {
	Topic         string
	Writer        sarama.SyncProducer
	WriterTimeout time.Duration
}

var _ workflow.EventSender = (*Sender)(nil)

func (p *Sender) Send(ctx context.Context, foreignID string, statusType int, headers map[workflow.Header]string) error {
	for ctx.Err() == nil {
		var kHeaders []sarama.RecordHeader
		for key, value := range headers {
			kHeaders = append(kHeaders, sarama.RecordHeader{
				Key:   []byte(key),
				Value: []byte(value),
			})
		}

		_, _, err := p.Writer.SendMessage(
			&sarama.ProducerMessage{
				Topic:     p.Topic,
				Key:       sarama.StringEncoder(foreignID),
				Value:     sarama.StringEncoder(strconv.FormatInt(int64(statusType), 10)),
				Headers:   kHeaders,
				Timestamp: time.Time{},
			},
		)
		if err != nil && (errors.Is(err, sarama.ErrLeaderNotAvailable) || errors.Is(err, context.DeadlineExceeded)) {
			time.Sleep(time.Millisecond * 100)
			continue
		} else if err != nil {
			return err
		}

		break
	}

	return ctx.Err()
}

func (p *Sender) Close() error {
	return p.Writer.Close()
}

func (s StreamConstructor) NewReceiver(
	ctx context.Context,
	topic string,
	name string,
	opts ...workflow.ReceiverOption,
) (workflow.EventReceiver, error) {
	var copts workflow.ReceiverOptions
	for _, opt := range opts {
		opt(&copts)
	}

	consumerConfig := *s.sharedConfig
	if copts.PollFrequency != 0 {
		consumerConfig.Consumer.MaxWaitTime = copts.PollFrequency
		consumerConfig.Consumer.Retry.Backoff = copts.PollFrequency
	}
	consumerConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	if copts.StreamFromLatest {
		consumerConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	}

	cg, err := sarama.NewConsumerGroup(s.brokers, name, &consumerConfig)
	if err != nil {
		return nil, err
	}

	consumeCtx, cancel := context.WithCancel(ctx)
	processor := newMessageProcessor(consumeCtx)
	go func() {
		for ctx.Err() == nil {
			err := cg.Consume(consumeCtx, []string{topic}, processor)
			if err != nil && errors.Is(err, context.Canceled) {
				// Exit on context cancellation
				return
			} else if err != nil {
				slog.Error("kafka consumer exited unexpectedly", "error", err.Error())

				err = wait(ctx, time.Second)
				if err != nil {
					return
				}

				continue
			}

			err = wait(ctx, time.Millisecond*250)
			if err != nil {
				return
			}
		}
	}()

	// Wait for the processor to be ready
	<-processor.ready

	return &Receiver{
		cancel:       cancel,
		topic:        topic,
		name:         name,
		msgProcessor: processor,
		options:      copts,
	}, nil
}

type Receiver struct {
	cancel       context.CancelFunc
	topic        string
	name         string
	msgProcessor *msgProcessor
	options      workflow.ReceiverOptions
}

func (r *Receiver) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case next := <-r.msgProcessor.iterator:
			return next()
		}
	}

	return nil, nil, ctx.Err()
}

func (r *Receiver) Close() error {
	r.cancel()
	return nil
}

var _ workflow.EventReceiver = (*Receiver)(nil)

func newMessageProcessor(ctx context.Context) *msgProcessor {
	return &msgProcessor{
		ctx:      ctx,
		ready:    make(chan bool, 1),
		iterator: make(chan func() (*workflow.Event, workflow.Ack, error)),
	}
}

// msgProcessor implements the sarama.ConsumerGroupHandler interface
type msgProcessor struct {
	ctx      context.Context
	ready    chan bool
	iterator chan func() (*workflow.Event, workflow.Ack, error)
}

func (mp *msgProcessor) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (mp *msgProcessor) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim processes messages from Kafka
func (mp *msgProcessor) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	mp.ready <- true

	for {
		select {
		case m := <-claim.Messages():
			select {
			case mp.iterator <- consume(session, m):
			case <-mp.ctx.Done():
				return mp.ctx.Err()
			}
		case <-mp.ctx.Done():
			return nil
		case <-session.Context().Done():
			return nil
		}
	}
}

func consume(
	session sarama.ConsumerGroupSession,
	m *sarama.ConsumerMessage,
) func() (*workflow.Event, workflow.Ack, error) {
	return func() (*workflow.Event, workflow.Ack, error) {
		statusType, err := strconv.ParseInt(string(m.Value), 10, 64)
		if err != nil {
			return nil, nil, err
		}

		headers := make(map[workflow.Header]string)
		for _, header := range m.Headers {
			headers[workflow.Header(header.Key)] = string(header.Value)
		}

		event := &workflow.Event{
			ID:        m.Offset + 1,
			ForeignID: string(m.Key),
			Type:      int(statusType),
			Headers:   headers,
			CreatedAt: m.Timestamp,
		}

		return event, func() error {
			session.MarkMessage(m, "")
			return nil
		}, nil
	}
}

// wait blocks until the provided duration elapses or the context is cancelled.
// If d is zero it returns nil immediately. If the context is cancelled before
// the timer fires the function returns ctx.Err(); otherwise it returns nil.
func wait(ctx context.Context, d time.Duration) error {
	if d == 0 {
		return nil
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
