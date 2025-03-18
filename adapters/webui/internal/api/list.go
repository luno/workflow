package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/luno/workflow"
)

type ListRequest struct {
	WorkflowName      string `json:"workflow_name"`
	Offset            int64  `json:"offset"`
	Limit             int    `json:"limit"`
	Order             string `json:"order"`
	FilterByForeignID string `json:"filter_by_foreign_id"`
	FilterByRunState  int    `json:"filter_by_run_state"`
	FilterByStatus    int    `json:"filter_by_status"`
}

type ListResponse struct {
	Items []ListItem `json:"items"`
}

// ListItem is a lightweight version of workflow.Record
type ListItem struct {
	WorkflowName   string    `json:"workflow_name"`
	ForeignID      string    `json:"foreign_id"`
	RunID          string    `json:"run_id"`
	RunState       string    `json:"run_state"`
	RunStateReason string    `json:"run_state_reason"`
	Status         string    `json:"status"`
	Version        uint      `json:"version"`
	TraceOrigin    string    `json:"trace_origin"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ListWorkflowRecords func(ctx context.Context, workflowName string, offset int64, limit int, order workflow.OrderType, filters ...workflow.RecordFilter) ([]workflow.Record, error)

func List(listRecords ListWorkflowRecords) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad Request: cannot read body", http.StatusBadRequest)
			return
		}

		var req ListRequest
		err = json.Unmarshal(body, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		order := workflow.OrderTypeAscending
		if req.Order == "desc" {
			order = workflow.OrderTypeDescending
		}

		var filters []workflow.RecordFilter
		if req.FilterByRunState != 0 {
			filters = append(filters, workflow.FilterByRunState(workflow.RunState(req.FilterByRunState)))
		}

		if req.FilterByForeignID != "" {
			filters = append(filters, workflow.FilterByForeignID(req.FilterByForeignID))
		}

		if req.FilterByStatus != 0 {
			filters = append(filters, workflow.FilterByStatus(req.FilterByStatus))
		}

		list, err := listRecords(r.Context(), req.WorkflowName, req.Offset, req.Limit, order, filters...)
		if err != nil {
			http.Error(w, "failed to collect records from store", http.StatusInternalServerError)
			return
		}

		var listItems []ListItem
		for _, record := range list {
			listItems = append(listItems, ListItem{
				WorkflowName:   record.WorkflowName,
				ForeignID:      record.ForeignID,
				RunID:          record.RunID,
				RunState:       record.RunState.String(),
				RunStateReason: record.Meta.RunStateReason,
				Status:         record.Meta.StatusDescription,
				Version:        record.Meta.Version,
				TraceOrigin:    record.Meta.TraceOrigin,
				CreatedAt:      record.CreatedAt,
				UpdatedAt:      record.UpdatedAt,
			})
		}

		resp := ListResponse{
			Items: listItems,
		}

		b, err := json.MarshalIndent(resp, " ", " ")
		if err != nil {
			http.Error(w, "failed to json marshal list of records", http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(b)
	}
}
