package errors

import (
	"errors"
	"slices"
	"strings"
)

type Error struct {
	err error

	// Meta contains fields that help pin down and find where the error originates and/or
	// what it relates to.
	Meta map[string]string
}

func Wrap(err error, msg string, meta map[string]string) *Error {
	return &Error{
		err:  errors.Join(errors.New(msg), err),
		Meta: meta,
	}
}

func (e *Error) Error() string {
	var sb strings.Builder
	sb.WriteString(e.err.Error())

	keys := make([]string, 0, len(e.Meta))
	for key := range e.Meta {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	for i, key := range keys {
		if i > 0 {
			sb.WriteString(", ")
		}

		sb.WriteString(key)
		sb.WriteString(": ")
		sb.WriteString(e.Meta[key])
	}

	return sb.String()
}
