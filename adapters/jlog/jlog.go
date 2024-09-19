package jlog

import (
	"context"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/jettison/log"
	"github.com/luno/workflow"
)

func New() *logger {
	return &logger{}
}

type logger struct{}

func (l logger) Debug(ctx context.Context, msg string, meta map[string]string) {
	log.Debug(ctx, msg, j.MKS(meta))
}

func (l logger) Error(ctx context.Context, err error) {
	log.Error(ctx, errors.Wrap(err, ""))
}

var _ workflow.Logger = (*logger)(nil)
