package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockConnectorConsumer struct {
	closed    bool
	closeErr  error
}

func (m *mockConnectorConsumer) Recv(ctx context.Context) (*ConnectorEvent, Ack, error) {
	return nil, nil, errors.New("not implemented")
}

func (m *mockConnectorConsumer) Close() error {
	m.closed = true
	return m.closeErr
}

func Test_connectorStreamer_Close(t *testing.T) {
	t.Run("delegates to consumer.Close", func(t *testing.T) {
		mock := &mockConnectorConsumer{}
		cs := connectorStreamer{
			consumer: mock,
		}

		err := cs.Close()

		require.NoError(t, err)
		require.True(t, mock.closed, "expected consumer.Close() to be called")
	})

	t.Run("propagates close error", func(t *testing.T) {
		closeErr := errors.New("close failed")
		mock := &mockConnectorConsumer{closeErr: closeErr}
		cs := connectorStreamer{
			consumer: mock,
		}

		err := cs.Close()

		require.ErrorIs(t, err, closeErr)
		require.True(t, mock.closed)
	})
}
