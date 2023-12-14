package workflow

import (
	"context"
)

type Cursor interface {
	Get(ctx context.Context, name string) (string, error)
	Set(ctx context.Context, name, value string) error
}
