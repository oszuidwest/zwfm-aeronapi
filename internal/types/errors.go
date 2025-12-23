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

func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s met ID '%s' niet gevonden", e.Resource, e.ID)
	}
	return fmt.Sprintf("%s niet gevonden", e.Resource)
}

func (e *NotFoundError) StatusCode() int { return http.StatusNotFound }

// NewNotFoundError creates a NotFoundError for missing resources.
func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{Resource: resource, ID: id}
}

// ValidationError indicates input validation failed.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func (e *ValidationError) StatusCode() int { return http.StatusBadRequest }

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// OperationError indicates a runtime operation failed.
type OperationError struct {
	Operation string
	Err       error
}

func (e *OperationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s mislukt: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("%s mislukt", e.Operation)
}

func (e *OperationError) Unwrap() error {
	return e.Err
}

func (e *OperationError) StatusCode() int { return http.StatusInternalServerError }

// NewOperationError creates a new OperationError.
func NewOperationError(operation string, err error) *OperationError {
	return &OperationError{Operation: operation, Err: err}
}

// ConfigError indicates invalid configuration.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuratie fout: %s - %s", e.Field, e.Message)
}

func (e *ConfigError) StatusCode() int { return http.StatusInternalServerError }

// NewConfigError creates a new ConfigError.
func NewConfigError(field, message string) *ConfigError {
	return &ConfigError{Field: field, Message: message}
}

// NewNoImageError creates a NotFoundError for entities without images.
// This is a convenience function for the common "no image" case.
func NewNoImageError(entity, id string) *NotFoundError {
	return &NotFoundError{Resource: entity + " afbeelding", ID: id}
}
