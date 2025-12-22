// Package types provides shared type definitions used across the application.
package types

import "fmt"

// NotFoundError indicates an entity was not found
type NotFoundError struct {
	Entity string // "artiest", "track", "backup"
	ID     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s met ID '%s' bestaat niet", e.Entity, e.ID)
}

// NoImageError indicates an entity has no image
type NoImageError struct {
	Entity string
	ID     string
}

func (e *NoImageError) Error() string {
	return fmt.Sprintf("%s met ID '%s' heeft geen afbeelding", e.Entity, e.ID)
}

// ValidationError indicates input validation failed
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ImageProcessingError indicates image processing failed
type ImageProcessingError struct {
	Message string
}

func (e *ImageProcessingError) Error() string {
	return fmt.Sprintf("afbeelding verwerking mislukt: %s", e.Message)
}

// DatabaseError wraps database operation errors
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

// ConfigurationError indicates invalid configuration
type ConfigurationError struct {
	Field   string
	Message string
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuratie fout: %s - %s", e.Field, e.Message)
}

// BackupError indicates backup operation failed
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

// Helper constructor functions

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(entity, id string) *NotFoundError {
	return &NotFoundError{Entity: entity, ID: id}
}

// NewNoImageError creates a new NoImageError
func NewNoImageError(entity, id string) *NoImageError {
	return &NoImageError{Entity: entity, ID: id}
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// NewImageProcessingError creates a new ImageProcessingError
func NewImageProcessingError(message string) *ImageProcessingError {
	return &ImageProcessingError{Message: message}
}

// NewDatabaseError creates a new DatabaseError
func NewDatabaseError(operation string, err error) *DatabaseError {
	return &DatabaseError{Operation: operation, Err: err}
}

// NewConfigurationError creates a new ConfigurationError
func NewConfigurationError(field, message string) *ConfigurationError {
	return &ConfigurationError{Field: field, Message: message}
}

// NewBackupError creates a new BackupError
func NewBackupError(operation string, err error) *BackupError {
	return &BackupError{Operation: operation, Err: err}
}
