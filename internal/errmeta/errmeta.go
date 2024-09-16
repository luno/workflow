package errmeta

func New(err error, meta map[string]string) error {
	return &ErrMeta{
		err:      err,
		metadata: meta,
	}
}

// ErrMeta represents an error with attached metadata for additional runtime context
type ErrMeta struct {
	err      error
	metadata map[string]string
}

// Error ensures compatibility with the stdlib error interface.
func (e *ErrMeta) Error() string {
	return e.err.Error()
}

// Unwrap allows errors wrapped by ErrMeta to be compatible with stdlib error unwrapping.
func (e *ErrMeta) Unwrap() error {
	return e.err
}

// Meta allows access to the metadata
func (e *ErrMeta) Meta() map[string]string {
	return e.metadata
}
