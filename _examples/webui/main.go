package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/luno/workflow"
	"github.com/luno/workflow/adapters/memrecordstore"
	"github.com/luno/workflow/adapters/memrolescheduler"
	"github.com/luno/workflow/adapters/memstreamer"
	"github.com/luno/workflow/adapters/webui"
)

func main() {
	recordStore := memrecordstore.New()
	w := Example(recordStore)
	ctx := context.Background()
	w.Run(ctx)
	defer w.Stop()

	seed := []string{
		"Customer 1",
		"Customer 2",
		"Customer 3",
	}
	for _, foreignID := range seed {
		_, err := w.Trigger(ctx, foreignID)
		if err != nil {
			panic(err)
		}
	}

	rec, err := recordStore.Latest(ctx, ExampleWorkflowName, "Customer 2")
	if err != nil {
		panic(err)
	}

	fmt.Println("Trace: ")
	fmt.Println(rec.Meta.TraceOrigin)

	paths := webui.Paths{
		List:       "/api/v1/list",
		Update:     "/api/v1/record/update",
		ObjectData: "/api/v1/record/objectdata",
	}
	http.HandleFunc("/", webui.HomeHandlerFunc(paths))
	http.HandleFunc(paths.List, webui.ListHandlerFunc(
		recordStore,
		func(workflowName string, enumValue int) string {
			if workflowName == ExampleWorkflowName {
				return Status(enumValue).String()
			}

			return strconv.Itoa(enumValue)
		},
	))
	http.HandleFunc(paths.ObjectData, webui.ObjectDataHandlerFunc(recordStore))
	http.HandleFunc(paths.Update, webui.UpdateHandlerFunc(recordStore))

	fmt.Println("Head on over to 'http://localhost:9492' to view!")

	err = http.ListenAndServe("localhost:9492", nil)
	if err != nil {
		panic(err)
	}

	os.Exit(0)
}

type ExampleData struct {
	Name      string
	Age       int
	City      string
	Languages []string
	Details   Details
}

type Details struct {
	Hobby      string
	Profession string
	Experience Experience
}

type Experience struct {
	Years    int
	Projects []string
}

type Status int

const (
	StatusUnknown Status = 0
	StatusStart   Status = 1
	StatusMiddle  Status = 2
	StatusEnd     Status = 3
)

func (s Status) String() string {
	switch s {
	case StatusStart:
		return "Start"
	case StatusMiddle:
		return "Middle"
	case StatusEnd:
		return "End"
	default:
		return "Unknown"
	}
}

const ExampleWorkflowName = "example"

func Example(rs workflow.RecordStore) *workflow.Workflow[ExampleData, Status] {
	b := workflow.NewBuilder[ExampleData, Status](ExampleWorkflowName)
	b.AddStep(
		StatusStart,
		func(ctx context.Context, r *workflow.Run[ExampleData, Status]) (Status, error) {
			return StatusMiddle, nil
		},
		StatusMiddle,
	)
	b.AddStep(
		StatusMiddle,
		func(ctx context.Context, r *workflow.Run[ExampleData, Status]) (Status, error) {
			if r.ForeignID == "Customer 2" {
				return r.Pause(ctx, "Always pause customer 2")
			}

			if r.ForeignID == "Customer 1" {
				return r.Cancel(ctx, "Always cancel Customer 1")
			}

			*r.Object = ExampleData{
				Name:      "Andrew",
				City:      "New York",
				Languages: []string{"Go", "C++", "Typescript"},
				Details: Details{
					Hobby:      "Coding",
					Profession: "Software Crafting",
					Experience: Experience{
						Years: 1_000_000,
						Projects: []string{
							"Workflow",
						},
					},
				},
			}

			return StatusEnd, nil
		},
		StatusEnd,
	)

	b.OnComplete(func(ctx context.Context, record *workflow.TypedRecord[ExampleData, Status]) error {
		fmt.Println("Completed: ", record.ForeignID)
		return nil
	})

	b.OnCancel(func(ctx context.Context, record *workflow.TypedRecord[ExampleData, Status]) error {
		fmt.Println("Cancelled: ", record.ForeignID)
		return nil
	})

	b.OnPause(func(ctx context.Context, record *workflow.TypedRecord[ExampleData, Status]) error {
		fmt.Println("Paused: ", record.ForeignID)
		return nil
	})

	return b.Build(
		memstreamer.New(),
		rs,
		memrolescheduler.New(),
	)
}
