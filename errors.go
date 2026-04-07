package agentmem

import "errors"

var (
	// ErrNotFound is returned when an entry is not found in the store.
	ErrNotFound = errors.New("entry not found")

	// ErrStoreClosed is returned when an operation is attempted on a closed store.
	ErrStoreClosed = errors.New("store is closed")

	// ErrEmptyKey is returned when an empty key is provided.
	ErrEmptyKey = errors.New("key must not be empty")

	// ErrNilValue is returned when a nil value is provided.
	ErrNilValue = errors.New("value must not be nil")
)
