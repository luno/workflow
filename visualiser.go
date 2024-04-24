package workflow

import (
	"os"
	"strings"
	"text/template"
)

func MermaidDiagram[Type any, Status StatusType](w *Workflow[Type, Status], path string, d MermaidDirection) error {
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
		starting = append(starting, Status(node).String())
	}

	var terminal []string
	for _, node := range graphInfo.TerminalNodes {
		terminal = append(terminal, Status(node).String())
	}

	var transitions []MermaidTransition
	for _, transition := range graphInfo.Transitions {
		transitions = append(transitions, MermaidTransition{
			From: Status(transition.From).String(),
			To:   Status(transition.To).String(),
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
	{{range $key, $value := .StartingPoints }}
	[*]-->{{$value}}
	{{- end }}
	{{range $key, $value := .Transitions }}
	{{$value.From}}-->{{$value.To}}
	{{- end }}
	{{range $key, $value := .TerminalPoints }}
	{{$value}}-->[*]
	{{- end }}
`
