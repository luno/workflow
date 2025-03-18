package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/luno/workflow"
)

type UpdateRequest struct {
	RunID  string `json:"run_id"`
	Action string `json:"action"`
}

func Update(store workflow.RecordStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			// NoReturnErr: HTTP api.
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		var req UpdateRequest
		err = json.Unmarshal(body, &req)
		if err != nil {
			http.Error(w, "Bad Request: cannot unmarshal body", http.StatusBadRequest)
			return
		}

		wr, err := store.Lookup(r.Context(), req.RunID)
		if err != nil {
			http.Error(w, "failed to lookup record from store", http.StatusInternalServerError)
			return
		}

		ctr := workflow.NewRunStateController(store.Store, wr)
		if err != nil {
			http.Error(w, "failed to build controller for record", http.StatusInternalServerError)
			return
		}

		switch req.Action {
		case "pause":
			err = ctr.Pause(r.Context(), "")
			if err != nil {
				http.Error(w, "failed to pause record", http.StatusInternalServerError)
				return
			}
		case "resume":
			err = ctr.Resume(r.Context())
			if err != nil {
				http.Error(w, "failed to resume record", http.StatusInternalServerError)
				return
			}
		case "cancel":
			err = ctr.Cancel(r.Context(), "")
			if err != nil {
				http.Error(w, "failed to cancel record", http.StatusInternalServerError)
				return
			}
		case "delete":
			err = ctr.DeleteData(r.Context(), "")
			if err != nil {
				http.Error(w, "failed to delete data of record", http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "unknown action provided", http.StatusInternalServerError)
		}
	}
}
