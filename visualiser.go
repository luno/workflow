package workflow

import (
	"errors"
	"os"
	"strings"
	"text/template"
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

	var starting []string
	for _, node := range graphInfo.StartingNodes {
		starting = append(starting, statusToString(Status(node)))
	}

	var terminal []string
	for _, node := range graphInfo.TerminalNodes {
		terminal = append(terminal, statusToString(Status(node)))
	}

	var transitions []MermaidTransition
	for _, transition := range graphInfo.Transitions {
		transitions = append(transitions, MermaidTransition{
			From: statusToString(Status(transition.From)),
			To:   statusToString(Status(transition.To)),
		})
	}

	mf := MermaidFormat{
		WorkflowName:   w.Name,
		Direction:      d,
		StartingPoints: starting,
		TerminalPoints: terminal,
		Transitions:    transitions,
	}

	return template.Must(template.New("").Parse("```"+mermaidTemplate+"```")).Execute(file, mf)
}

func statusToString[Status StatusType](s Status) string {
	str := strings.ToLower(s.String())
	str = strings.Replace(str, " ", "_", -1)
	return str
}

type MermaidFormat struct {
	WorkflowName   string
	Direction      MermaidDirection
	StartingPoints []string
	TerminalPoints []string
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
	From string
	To   string
}

var mermaidTemplate = `mermaid
---
title: Diagram of {{.WorkflowName}} Workflow
---
stateDiagram-v2
	direction {{.Direction}}
	{{range $key, $value := .Transitions }}
	{{$value.From}}-->{{$value.To}}
	{{- end }}
`
