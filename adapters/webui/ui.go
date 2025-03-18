package webui

import (
	"net/http"

	"github.com/luno/workflow"

	"github.com/luno/workflow/adapters/webui/internal/api"
	"github.com/luno/workflow/adapters/webui/internal/frontend"
)

func HomeHandlerFunc(paths Paths) http.HandlerFunc {
	return frontend.HomeHandlerFunc(paths)
}

type (
	Stringer            = api.Stringer
	ListWorkflowRecords = api.ListWorkflowRecords
	LookupFn            = api.LookupFn
	Paths               = frontend.Paths
)

func ListHandlerFunc(store workflow.RecordStore) http.HandlerFunc {
	return api.List(store.List)
}

func ObjectDataHandlerFunc(store workflow.RecordStore) http.HandlerFunc {
	return api.ObjectData(store.Lookup)
}

func UpdateHandlerFunc(store workflow.RecordStore) http.HandlerFunc {
	return api.Update(store)
}
