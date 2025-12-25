package repository

import "errors"

// Common repository errors.
// These errors provide a consistent error interface across different storage implementations.
var (
	// ErrNotFound indicates the requested entity was not found.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates an entity with the same identifier already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrConcurrentUpdate indicates a concurrent update conflict (optimistic locking failure).
	// This error signals that a retry is needed.
	ErrConcurrentUpdate = errors.New("concurrent update detected")
)
