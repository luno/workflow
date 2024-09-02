package errors

import (
	"errors"
)

// Error is a trivial implementation of error that allows key value
// metadata to accompany the error along with support for wrapping errors.
// Error's Error method formats the wrapped errors and collected metadata.
// Note that the metadata is kept at a single level and thus keys need to
// be unique if they are not to over-written.
type Error struct {
	msg string

	// child relates to the next error in the linked-list of errors or "chain" or errors.
	child error

	// meta contains fields that help pin down and find where the error originates and/or
	// what it relates to.
	meta map[string]string
}

func (e *Error) Unwrap() error {
	return e.child
}

func (e *Error) Is(target error) bool {
	if target == nil {
		return e.msg == "" && e.child == nil
	}

	return e.msg == target.Error()
}

func New(msg string) error {
	if msg == "" {
		return nil
	}

	return &Error{
		msg:  msg,
		meta: make(map[string]string),
	}
}

func Wrap(err error, msg string) error {
	return WrapWithMeta(err, msg, map[string]string{})
}

func WrapWithMeta(err error, msg string, meta map[string]string) error {
	if err == nil {
		return nil
	}

	e, isInternalError := err.(*Error)
	if isInternalError {
		var newErr Error
		newErr.meta = make(map[string]string)
		for k, v := range meta {
			newErr.meta[k] = v
		}

		for k, v := range e.meta {
			newErr.meta[k] = v
		}

		newErr.msg = msg
		newErr.child = err
		// metadata is kept as a single copy at the top level to avoid needing to traverse the tree to collect
		// the meta key value pairs. The trade-off here is losing record of where the key value pairs originate from
		// and having a top level namespacing restriction.
		e.meta = nil

		return &newErr
	}

	return &Error{
		msg:   msg,
		child: err,
		meta:  meta,
	}
}

func (e *Error) Error() string {
	if e.child == nil && e.msg != "" {
		return e.msg
	}

	if e.msg == "" {
		return e.child.Error()
	}

	return errors.Join(errors.New(e.msg), e.child).Error()
}

func (e *Error) Meta() map[string]string {
	return e.meta
}
