package domain

import "errors"

var (
	ErrNotFound             = errors.New("not found")
	ErrAlreadyExists        = errors.New("already exists")
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrConcurrencyConflict  = errors.New("concurrency conflict")
	ErrDelegationCycle      = errors.New("delegation cycle detected")
	ErrScopeExpansion       = errors.New("scope expansion not allowed")
	ErrIdempotencyViolation = errors.New("idempotency violation")
	ErrContextCanceled      = errors.New("context canceled")
	ErrTimeout              = errors.New("timeout")
)
