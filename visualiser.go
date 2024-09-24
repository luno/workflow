package workflow

import (
	"errors"
	"os"
	"strings"
	"text/template"

	"github.com/luno/workflow/internal/util"
)

// CreateDiagram creates a diagram in a md file for communicating a workflow's set of steps in an easy-to-understand
// manner.
func CreateDiagram[Type any, Status StatusType](a API[Type, Status], path string, d MermaidDirection) error {
	return mermaidDiagram[Type, Status](a, path, d)
}

func mermaidDiagram[Type any, Status StatusType](a API[Type, Status], path string, d MermaidDirection) error {
	w, ok := a.(*Workflow[Type, Status])
	if !ok {
		return errors.New("cannot create diagram for non-original workflow.Workflow type")
	}

	breakDown := strings.Split(path, "/")
	dirPath := strings.Join(breakDown[:len(breakDown)-1], "/")

	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return err
	}

	fileName := breakDown[len(breakDown)-1:][0]
	file, err := os.Create(dirPath + "/" + fileName)
	if err != nil {
		return err
	}

	if d == UnknownDirection {
		d = LeftToRightDirection
	}

	graphInfo := w.statusGraph.Info()

	var starting []int
	for _, node := range graphInfo.StartingNodes {
		starting = append(starting, node)
	}

	var terminal []int
	for _, node := range graphInfo.TerminalNodes {
		terminal = append(terminal, node)
	}

	var transitions []MermaidTransition
	for _, transition := range graphInfo.Transitions {
		transitions = append(transitions, MermaidTransition{
			From: transition.From,
			To:   transition.To,
		})
	}

	mf := MermaidFormat{
		WorkflowName:   w.Name,
		Direction:      d,
		StartingPoints: starting,
		TerminalPoints: terminal,
		Transitions:    transitions,
		Nodes:          w.statusGraph.Nodes(),
	}

	return template.Must(template.New("").Funcs(map[string]any{
		"Description": description[Status],
	}).Parse("```"+mermaidTemplate+"```")).Execute(file, mf)
}

func description[Status StatusType](val int) string {
	s := Status(val).String()
	return util.CamelCaseToSpacing(s)
}

type MermaidFormat struct {
	WorkflowName   string
	Direction      MermaidDirection
	Nodes          []int
	StartingPoints []int
	TerminalPoints []int
	Transitions    []MermaidTransition
}

type MermaidDirection string

const (
	UnknownDirection     MermaidDirection = ""
	TopToBottomDirection MermaidDirection = "TB"
	LeftToRightDirection MermaidDirection = "LR"
	RightToLeftDirection MermaidDirection = "RL"
	BottomToTopDirection MermaidDirection = "BT"
)

type MermaidTransition struct {
	From int
	To   int
}

var mermaidTemplate = `mermaid
---
title: Diagram of {{.WorkflowName}} Workflow
---
stateDiagram-v2
	direction {{.Direction}}
	{{range $key, $value := .Nodes }}
	{{$value}}: {{Description $value}}
	{{- end }}

	{{range $key, $value := .Transitions }}
	{{$value.From}}-->{{$value.To}}
	{{- end }}
`
