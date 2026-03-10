package wredis_test

import (
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/luno/workflow/adapters/wredis"
)

func TestRedisRecordStore(t *testing.T) {
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})

	factory := func() workflow.RecordStore {
		// Clean the database before each test
		err := client.FlushDB(ctx).Err()
		require.NoError(t, err)

		return wredis.New(client)
	}

	adaptertest.RunRecordStoreTest(t, factory)
}

func TestDeleteOutboxEvent_NotFound(t *testing.T) {
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})

	store := wredis.New(client)

	// Deleting a non-existent event should not error (reverse index miss).
	err = store.DeleteOutboxEvent(ctx, "non-existent-id")
	require.NoError(t, err)
}

func TestDeleteOutboxEvent_DoubleDelete(t *testing.T) {
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})

	store := wredis.New(client)

	// Store a record to create an outbox event.
	record := &workflow.Record{
		WorkflowName: "test-workflow",
		ForeignID:    "foreign-1",
		RunID:        "run-1",
		RunState:     1,
		Status:       1,
		Object:       []byte(`{}`),
	}
	err = store.Store(ctx, record)
	require.NoError(t, err)

	// List the outbox events to get the ID.
	events, err := store.ListOutboxEvents(ctx, "test-workflow", 10)
	require.NoError(t, err)
	require.Len(t, events, 1)

	eventID := events[0].ID

	// First delete should succeed.
	err = store.DeleteOutboxEvent(ctx, eventID)
	require.NoError(t, err)

	// Second delete should also succeed (idempotent — reverse index cleaned up,
	// so this hits the redis.Nil path).
	err = store.DeleteOutboxEvent(ctx, eventID)
	require.NoError(t, err)

	// Outbox should be empty.
	events, err = store.ListOutboxEvents(ctx, "test-workflow", 10)
	require.NoError(t, err)
	require.Empty(t, events)
}

func TestDeleteOutboxEvent_StaleReverseIndex(t *testing.T) {
	ctx := t.Context()

	redisInstance, err := rediscontainer.Run(ctx, "redis:7-alpine")
	testcontainers.CleanupContainer(t, redisInstance)
	require.NoError(t, err)

	host, err := redisInstance.Host(ctx)
	require.NoError(t, err)

	port, err := redisInstance.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
	})

	// Simulate a stale reverse index: the reverse key exists but the event
	// is no longer in the outbox list (e.g. manually removed or race condition).
	outboxKey := "workflow:outbox:test-workflow"
	reverseKey := "workflow:outbox-reverse:stale-event-id"
	err = client.Set(ctx, reverseKey, outboxKey, 0).Err()
	require.NoError(t, err)

	store := wredis.New(client)

	// Should succeed — the Lua script returns 0 (not found in list) which is
	// handled gracefully.
	err = store.DeleteOutboxEvent(ctx, "stale-event-id")
	require.NoError(t, err)

	// Reverse index should be cleaned up even though the event wasn't in the list.
	exists, err := client.Exists(ctx, reverseKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)
}
