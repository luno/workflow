package kafkastreamer

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/luno/workflow"
	"github.com/segmentio/kafka-go"
)

func New(brokers []string) *StreamConstructor {
	return &StreamConstructor{
		brokers: brokers,
	}
}

var _ workflow.EventStreamer = (*StreamConstructor)(nil)

type StreamConstructor struct {
	brokers []string
}

func (s StreamConstructor) NewSender(ctx context.Context, topic string) (workflow.EventSender, error) {
	return &Sender{
		Topic: topic,
		Writer: &kafka.Writer{
			Addr:                   kafka.TCP(s.brokers...),
			Topic:                  topic,
			AllowAutoTopicCreation: true,
			RequiredAcks:           kafka.RequireOne,
		},
		WriterTimeout: time.Second * 10,
	}, nil
}

type Sender struct {
	Topic         string
	Writer        *kafka.Writer
	WriterTimeout time.Duration
}

var _ workflow.EventSender = (*Sender)(nil)

func (p *Sender) Send(ctx context.Context, foreignID string, statusType int, headers map[workflow.Header]string) error {
	for ctx.Err() == nil {
		ctx, cancel := context.WithTimeout(ctx, p.WriterTimeout)
		defer cancel()

		var kHeaders []kafka.Header
		for key, value := range headers {
			kHeaders = append(kHeaders, kafka.Header{
				Key:   string(key),
				Value: []byte(value),
			})
		}

		msg := kafka.Message{
			Key:     []byte(foreignID),
			Value:   []byte(strconv.FormatInt(int64(statusType), 10)),
			Headers: kHeaders,
		}

		err := p.Writer.WriteMessages(ctx, msg)
		if errors.Is(err, kafka.LeaderNotAvailable) || errors.Is(err, context.DeadlineExceeded) {
			time.Sleep(time.Millisecond * 250)
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

	startOffset := kafka.FirstOffset

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        s.brokers,
		GroupID:        name,
		Topic:          topic,
		ReadBackoffMin: copts.PollFrequency,
		ReadBackoffMax: copts.PollFrequency,
		StartOffset:    startOffset,
		QueueCapacity:  1000,
		MinBytes:       10,  // 10B
		MaxBytes:       1e9, // 9MB
		MaxWait:        time.Second,
	})

	return &Receiver{
		topic:   topic,
		name:    name,
		reader:  kafkaReader,
		options: copts,
	}, nil
}

type Receiver struct {
	topic   string
	name    string
	reader  *kafka.Reader
	options workflow.ReceiverOptions
}

func (r *Receiver) Recv(ctx context.Context) (*workflow.Event, workflow.Ack, error) {
	var commit []kafka.Message
	for ctx.Err() == nil {
		m, err := r.reader.FetchMessage(ctx)
		if err != nil {
			return nil, nil, err
		}

		// Append the message to the commit slice to ensure we send all messages that have been processed
		commit = append(commit, m)

		statusType, err := strconv.ParseInt(string(m.Value), 10, 64)
		if err != nil {
			return nil, nil, err
		}

		headers := make(map[workflow.Header]string)
		for _, header := range m.Headers {
			headers[workflow.Header(header.Key)] = string(header.Value)
		}

		event := &workflow.Event{
			ID:        m.Offset,
			ForeignID: string(m.Key),
			Type:      int(statusType),
			Headers:   headers,
			CreatedAt: m.Time,
		}

		return event,
			func() error {
				return r.reader.CommitMessages(ctx, commit...)
			},
			nil
	}

	return nil, nil, ctx.Err()
}

func (r *Receiver) Close() error {
	return r.reader.Close()
}

var _ workflow.EventReceiver = (*Receiver)(nil)
