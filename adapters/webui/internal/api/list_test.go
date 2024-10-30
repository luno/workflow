package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/luno/workflow"
	"github.com/stretchr/testify/require"

	"github.com/luno/workflow/adapters/webui/internal/api"
)

func TestListHandler(t *testing.T) {
	testCases := []struct {
		name               string
		request            api.ListRequest
		listResponse       []workflow.Record
		expectedResponse   api.ListResponse
		expectedStatusCode int
	}{
		{
			name: "Golden path - asc",
			request: api.ListRequest{
				WorkflowName: "test",
				Limit:        5,
				Order:        "asc",
			},
			listResponse: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunState:     workflow.RunStateCompleted,
					Status:       9,
				},
			},
			expectedResponse: api.ListResponse{
				Items: []api.ListItem{
					{
						WorkflowName: "test",
						ForeignID:    "9",
						RunState:     workflow.RunStateCompleted.String(),
						Status:       "9",
					},
				},
			},
			expectedStatusCode: 200,
		},
		{
			name: "Golden path - desc",
			request: api.ListRequest{
				WorkflowName: "test",
				Offset:       0,
				Limit:        5,
				Order:        "desc",
			},
			listResponse: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunState:     workflow.RunStateCompleted,
					Status:       9,
				},
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunState:     workflow.RunStateCompleted,
					Status:       9,
				},
			},
			expectedResponse: api.ListResponse{
				Items: []api.ListItem{
					{
						WorkflowName: "test",
						ForeignID:    "9",
						RunState:     workflow.RunStateCompleted.String(),
						Status:       "9",
					},
					{
						WorkflowName: "test",
						ForeignID:    "9",
						RunState:     workflow.RunStateCompleted.String(),
						Status:       "9",
					},
				},
			},
			expectedStatusCode: 200,
		},
		{
			name: "Filter by RunState",
			request: api.ListRequest{
				WorkflowName:     "test",
				Offset:           0,
				Limit:            5,
				Order:            "desc",
				FilterByRunState: int(workflow.RunStateCompleted),
			},
			listResponse: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunState:     workflow.RunStateCompleted,
					Status:       9,
				},
			},
			expectedResponse: api.ListResponse{
				Items: []api.ListItem{
					{
						WorkflowName: "test",
						ForeignID:    "9",
						RunState:     workflow.RunStateCompleted.String(),
						Status:       "9",
					},
				},
			},
			expectedStatusCode: 200,
		},
		{
			name: "Filter by ForeignID",
			request: api.ListRequest{
				WorkflowName:      "test",
				Offset:            0,
				Limit:             5,
				Order:             "desc",
				FilterByForeignID: "9",
			},
			listResponse: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunState:     workflow.RunStateCompleted,
					Status:       9,
				},
			},
			expectedResponse: api.ListResponse{
				Items: []api.ListItem{
					{
						WorkflowName: "test",
						ForeignID:    "9",
						RunState:     workflow.RunStateCompleted.String(),
						Status:       "9",
					},
				},
			},
			expectedStatusCode: 200,
		},
		{
			name: "Filter by Status",
			request: api.ListRequest{
				WorkflowName:   "test",
				Offset:         0,
				Limit:          5,
				Order:          "desc",
				FilterByStatus: 9,
			},
			listResponse: []workflow.Record{
				{
					WorkflowName: "test",
					ForeignID:    "9",
					RunState:     workflow.RunStateCompleted,
					Status:       9,
				},
			},
			expectedResponse: api.ListResponse{
				Items: []api.ListItem{
					{
						WorkflowName: "test",
						ForeignID:    "9",
						RunState:     workflow.RunStateCompleted.String(),
						Status:       "9",
					},
				},
			},
			expectedStatusCode: 200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				listFn := func(ctx context.Context, workflowName string, offsetID int64, limit int, order workflow.OrderType, filters ...workflow.RecordFilter) ([]workflow.Record, error) {
					// Workflow's record store has its own tests and we don't need to test workflow's implementation
					// of listing with filters etc. The main thing to test would be to ensure the fields are mapped
					// correctly and end the concern there (hand off to the record store implementation).
					require.Equal(t, tc.request.WorkflowName, workflowName)
					require.Equal(t, tc.request.Offset, offsetID)
					require.Equal(t, tc.request.Limit, limit)
					require.Equal(t, tc.request.Order, order.String())

					filter := workflow.MakeFilter(filters...)
					if tc.request.FilterByRunState != 0 {
						require.True(t, filter.ByRunState().Enabled)
						require.Equal(t, fmt.Sprintf("%v", tc.request.FilterByRunState), filter.ByRunState().Value)
					}
					if tc.request.FilterByForeignID != "" {
						require.True(t, filter.ByForeignID().Enabled)
						require.Equal(t, tc.request.FilterByForeignID, filter.ByForeignID().Value)
					}
					if tc.request.FilterByStatus != 0 {
						require.True(t, filter.ByStatus().Enabled)
						require.Equal(t, fmt.Sprintf("%v", tc.request.FilterByStatus), filter.ByStatus().Value)
					}

					return tc.listResponse, nil
				}
				api.List(listFn, func(workflowName string, enumValue int) string {
					return "9"
				})(w, r)
			}))
			t.Cleanup(srv.Close)

			body, err := json.Marshal(tc.request)
			require.NoError(t, err)

			resp, err := http.Post(srv.URL, "application/json", bytes.NewReader(body))
			require.NoError(t, err)

			require.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			respBody, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var expectedResp api.ListResponse
			err = json.Unmarshal(respBody, &expectedResp)
			require.NoError(t, err)

			require.Equal(t, tc.expectedResponse, expectedResp)
		})
	}
}
