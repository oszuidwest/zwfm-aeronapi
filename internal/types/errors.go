// Package types provides shared type definitions used across the application.
package types

import (
	"fmt"
	"net/http"
)

// HTTPError is implemented by errors that map to HTTP status codes.
type HTTPError interface {
	error
	StatusCode() int
}

// NotFoundError indicates a resource was not found.
type NotFoundError struct {
	Resource string
	ID       string
}

// Error returns a formatted error message in Dutch.
func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s met ID '%s' niet gevonden", e.Resource, e.ID)
	}
	return fmt.Sprintf("%s niet gevonden", e.Resource)
}

// StatusCode returns HTTP 404 Not Found.
func (e *NotFoundError) StatusCode() int { return http.StatusNotFound }

// NewNotFoundError creates a NotFoundError for the specified resource type and ID.
func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{Resource: resource, ID: id}
}

// ValidationError indicates input validation failed.
type ValidationError struct {
	Field   string
	Message string
}

// Error returns the validation error message.
func (e *ValidationError) Error() string {
	return e.Message
}

// StatusCode returns HTTP 400 Bad Request.
func (e *ValidationError) StatusCode() int { return http.StatusBadRequest }

// NewValidationError creates a ValidationError for the specified field.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// OperationError indicates a runtime operation failed.
type OperationError struct {
	Operation string
	Err       error
}

// Error returns a formatted error message in Dutch.
func (e *OperationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s mislukt: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("%s mislukt", e.Operation)
}

// Unwrap returns the underlying error.
func (e *OperationError) Unwrap() error {
	return e.Err
}

// StatusCode returns HTTP 500 Internal Server Error.
func (e *OperationError) StatusCode() int { return http.StatusInternalServerError }

// NewOperationError creates an OperationError wrapping the given error.
func NewOperationError(operation string, err error) *OperationError {
	return &OperationError{Operation: operation, Err: err}
}

// ConflictError indicates a resource conflict (e.g., operation already running).
type ConflictError struct {
	Resource string
	Message  string
}

// Error returns the conflict error message.
func (e *ConflictError) Error() string {
	return e.Message
}

// StatusCode returns HTTP 409 Conflict.
func (e *ConflictError) StatusCode() int { return http.StatusConflict }

// NewConflictError creates a ConflictError for the specified resource.
func NewConflictError(resource, message string) *ConflictError {
	return &ConflictError{Resource: resource, Message: message}
}

// ConfigError indicates invalid configuration.
type ConfigError struct {
	Field   string
	Message string
}

// Error returns a formatted configuration error message in Dutch.
func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuratie fout: %s - %s", e.Field, e.Message)
}

// StatusCode returns HTTP 500 Internal Server Error.
func (e *ConfigError) StatusCode() int { return http.StatusInternalServerError }

// NewConfigError creates a ConfigError for the specified configuration field.
func NewConfigError(field, message string) *ConfigError {
	return &ConfigError{Field: field, Message: message}
}

// NewNoImageError creates a NotFoundError for entities without images.
func NewNoImageError(entity, id string) *NotFoundError {
	return &NotFoundError{Resource: entity + " afbeelding", ID: id}
}
