package rinkrolescheduler_test

import (
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/luno/rink/v2"
	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/adaptertest"
	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"

	"github.com/luno/workflow/adapters/rinkrolescheduler"
)

func TestRoleScheduler(t *testing.T) {
	config := clientv3.Config{
		Endpoints:            []string{"http://localhost:2380", "http://localhost:2379"},
		DialKeepAliveTime:    time.Second,
		DialKeepAliveTimeout: time.Second,
		DialTimeout:          time.Second,
		DialOptions:          []grpc.DialOption{grpc.WithBlock()},
	}
	cli, err := clientv3.New(config)
	require.NoError(t, err)
	t.Cleanup(func() {
		cli.Close()
	})

	adaptertest.RunRoleSchedulerTest(t, func(t *testing.T, instances int) []workflow.RoleScheduler {
		clusterID := strconv.Itoa(rand.Int())
		rinkInstances := make([]workflow.RoleScheduler, instances)
		for memberOfInstance := range instances {
			r := rink.New(cli, clusterID,
				rink.WithRolesOptions(rink.RolesOptions{
					AwaitRetry:  50 * time.Millisecond,
					LockTimeout: 50 * time.Millisecond,
				}),
				rink.WithClusterOptions(rink.ClusterOptions{
					MemberName:    strconv.Itoa(memberOfInstance),
					NewMemberWait: 50 * time.Millisecond,
				}),
			)

			go func() {
				r.Run()
			}()

			rinkInstances[memberOfInstance] = rinkrolescheduler.New(r)
		}

		return rinkInstances
	})
}
