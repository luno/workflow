package webui

import (
	"net/http"

	"github.com/luno/workflow"

	"github.com/luno/workflow/adapters/webui/internal/api"
	"github.com/luno/workflow/adapters/webui/internal/frontend"
)

func HomeHandlerFunc() http.HandlerFunc {
	return frontend.HomeHandlerFunc()
}

type (
	Stringer            = api.Stringer
	ListWorkflowRecords = api.ListWorkflowRecords
	LookupFn            = api.LookupFn
)

func ListHandlerFunc(store workflow.RecordStore, stringer Stringer) http.HandlerFunc {
	return api.List(store.List, stringer)
}

func ObjectDataHandlerFunc(store workflow.RecordStore) http.HandlerFunc {
	return api.ObjectData(store.Lookup)
}

func UpdateHandlerFunc(store workflow.RecordStore) http.HandlerFunc {
	return api.Update(store)
}
