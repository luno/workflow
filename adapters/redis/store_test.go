package redis

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
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
		client.FlushDB(ctx)
		return New(client)
	}

	adaptertest.RunRecordStoreTest(t, factory)
}
