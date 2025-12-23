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

// NotFoundError indicates an entity was not found.
type NotFoundError struct {
	Entity string
	ID     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s met ID '%s' bestaat niet", e.Entity, e.ID)
}

func (e *NotFoundError) StatusCode() int { return http.StatusNotFound }

// NoImageError indicates an entity has no image.
type NoImageError struct {
	Entity string
	ID     string
}

func (e *NoImageError) Error() string {
	return fmt.Sprintf("%s met ID '%s' heeft geen afbeelding", e.Entity, e.ID)
}

func (e *NoImageError) StatusCode() int { return http.StatusNotFound }

// ValidationError indicates input validation failed.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func (e *ValidationError) StatusCode() int { return http.StatusBadRequest }

// ImageProcessingError indicates image processing failed.
type ImageProcessingError struct {
	Message string
}

func (e *ImageProcessingError) Error() string {
	return fmt.Sprintf("afbeelding verwerking mislukt: %s", e.Message)
}

func (e *ImageProcessingError) StatusCode() int { return http.StatusBadRequest }

// DatabaseError wraps database operation errors.
type DatabaseError struct {
	Operation string
	Err       error
}

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("database fout bij %s: %v", e.Operation, e.Err)
}

func (e *DatabaseError) Unwrap() error {
	return e.Err
}

func (e *DatabaseError) StatusCode() int { return http.StatusInternalServerError }

// ConfigurationError indicates invalid configuration.
type ConfigurationError struct {
	Field   string
	Message string
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuratie fout: %s - %s", e.Field, e.Message)
}

func (e *ConfigurationError) StatusCode() int { return http.StatusInternalServerError }

// BackupError indicates backup operation failed.
type BackupError struct {
	Operation string
	Err       error
}

func (e *BackupError) Error() string {
	return fmt.Sprintf("backup %s mislukt: %v", e.Operation, e.Err)
}

func (e *BackupError) Unwrap() error {
	return e.Err
}

func (e *BackupError) StatusCode() int { return http.StatusInternalServerError }

// NewNotFoundError constructs a NotFoundError for missing entities.
func NewNotFoundError(entity, id string) *NotFoundError {
	return &NotFoundError{Entity: entity, ID: id}
}

// NewNoImageError constructs a NoImageError for entities without images.
func NewNoImageError(entity, id string) *NoImageError {
	return &NoImageError{Entity: entity, ID: id}
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}
