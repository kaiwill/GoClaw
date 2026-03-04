// Package types provides error types for the GoClaw agent runtime.
package types

import "errors"

var (
	// ErrNotFound is returned when a requested resource is not found
	ErrNotFound = errors.New("not found")

	// ErrUnauthorized is returned when authentication fails
	ErrUnauthorized = errors.New("unauthorized")

	// ErrRateLimited is returned when rate limit is exceeded
	ErrRateLimited = errors.New("rate limited")

	// ErrInvalidInput is returned for invalid input parameters
	ErrInvalidInput = errors.New("invalid input")

	// ErrTimeout is returned when an operation times out
	ErrTimeout = errors.New("timeout")

	// ErrNotSupported is returned when a capability is not supported
	ErrNotSupported = errors.New("not supported")
)

// ProviderCapabilityError represents an error when a requested capability is not supported
type ProviderCapabilityError struct {
	Provider  string
	Capability string
	Message   string
}

func (e *ProviderCapabilityError) Error() string {
	return "provider_capability_error provider=" + e.Provider +
		" capability=" + e.Capability +
		" message=" + e.Message
}

// StreamError represents errors that can occur during streaming
type StreamError struct {
	Type    string
	Message string
	Err     error
}

func (e *StreamError) Error() string {
	if e.Err != nil {
		return e.Type + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Type + ": " + e.Message
}

func (e *StreamError) Unwrap() error {
	return e.Err
}

const (
	StreamErrorTypeHTTP      = "http"
	StreamErrorTypeJSON      = "json"
	StreamErrorTypeInvalidSSE = "invalid_sse"
	StreamErrorTypeProvider  = "provider"
	StreamErrorTypeIO        = "io"
)

// NewHTTPStreamError creates a new HTTP stream error
func NewHTTPStreamError(message string, err error) *StreamError {
	return &StreamError{
		Type:    StreamErrorTypeHTTP,
		Message: message,
		Err:     err,
	}
}

// NewJSONStreamError creates a new JSON parse error
func NewJSONStreamError(message string, err error) *StreamError {
	return &StreamError{
		Type:    StreamErrorTypeJSON,
		Message: message,
		Err:     err,
	}
}

// NewInvalidSSEError creates a new invalid SSE format error
func NewInvalidSSEError(message string) *StreamError {
	return &StreamError{
		Type:    StreamErrorTypeInvalidSSE,
		Message: message,
	}
}

// NewProviderStreamError creates a new provider error
func NewProviderStreamError(message string) *StreamError {
	return &StreamError{
		Type:    StreamErrorTypeProvider,
		Message: message,
	}
}
