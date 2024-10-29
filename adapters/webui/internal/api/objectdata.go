package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/luno/workflow"
)

type ObjectDataRequest struct {
	RecordID int64 `json:"record_id"`
}

type LookupFn func(ctx context.Context, id int64) (*workflow.Record, error)

func ObjectData(lookup LookupFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad Request: cannot read body", http.StatusBadRequest)
			return
		}

		var req ObjectDataRequest
		err = json.Unmarshal(body, &req)
		if err != nil {
			http.Error(w, "Bad Request: cannot unmarshal body", http.StatusBadRequest)
			return
		}

		record, err := lookup(r.Context(), req.RecordID)
		if err != nil {
			http.Error(w, "failed to lookup record from store", http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(record.Object)
	}
}
