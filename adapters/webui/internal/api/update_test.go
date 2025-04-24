package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/webui/internal/api"
)

func TestUpdateHandler(t *testing.T) {
	testCases := []struct {
		name               string
		request            api.UpdateRequest
		before             []workflow.Record
		after              []workflow.Record
		expectedStatusCode int
	}{
		{
			name: "Pause",
			request: api.UpdateRequest{
				RunID:  "1",
				Action: "pause",
			},
			before: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStateRunning,
					Status:       2,
				},
			},
			after: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStatePaused,
					Status:       2,
				},
			},
			expectedStatusCode: 200,
		},
		{
			name: "Resume",
			request: api.UpdateRequest{
				RunID:  "1",
				Action: "resume",
			},
			before: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStatePaused,
					Status:       2,
				},
			},
			after: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStateRunning,
					Status:       2,
				},
			},
			expectedStatusCode: 200,
		},
		{
			name: "Cancel",
			request: api.UpdateRequest{
				RunID:  "1",
				Action: "cancel",
			},
			before: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStateRunning,
					Status:       2,
				},
			},
			after: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStateCancelled,
					Status:       2,
				},
			},
			expectedStatusCode: 200,
		},
		{
			name: "Delete",
			request: api.UpdateRequest{
				RunID:  "1",
				Action: "delete",
			},
			before: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStateCompleted,
					Status:       9,
				},
			},
			after: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStateRequestedDataDeleted,
					Status:       9,
					Object:       []byte("Deleted"),
				},
			},
			expectedStatusCode: 200,
		},
		{
			name: "Unknown action",
			request: api.UpdateRequest{
				RunID:  "1",
				Action: "",
			},
			before: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunID:        "1",
					RunState:     workflow.RunStateCompleted,
					Status:       9,
				},
			},
			expectedStatusCode: 500,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recordStore := memrecordstore.New()

			ctx := t.Context()
			for _, record := range tc.before {
				err := recordStore.Store(ctx, &record)
				require.NoError(t, err)
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				api.Update(recordStore)(w, r)
			}))
			t.Cleanup(srv.Close)

			body, err := json.Marshal(tc.request)
			require.NoError(t, err)

			resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
			require.NoError(t, err)

			require.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			for _, expected := range tc.after {
				actual, err := recordStore.Lookup(ctx, expected.RunID)
				require.NoError(t, err)

				// No need to compare objects
				expected.Object = actual.Object
				expected.Meta.Version = 1
				require.Equal(t, expected, *actual)
			}
		})
	}
}
