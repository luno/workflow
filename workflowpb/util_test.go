package workflowpb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/luno/workflow"
	"github.com/luno/workflow/workflowpb"
)

func TestProtoMarshalAndUnmarshal(t *testing.T) {
	now := time.Date(2024, time.April, 9, 0, 0, 0, 0, time.UTC)
	wireRecord := workflow.Record{
		WorkflowName: "example",
		ForeignID:    "o283u44092384",
		RunID:        "KJDSHFS-SDLFKNSD-EEWKHRCV",
		RunState:     workflow.RunStateCompleted,
		Status:       3,
		Object:       []byte("{}"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	protoBytes, err := workflowpb.ProtoMarshal(&wireRecord)
	require.NoError(t, err)

	deserialised, err := workflowpb.UnmarshalRecord(protoBytes)
	require.NoError(t, err)

	require.Equal(t, wireRecord, *deserialised)
}
