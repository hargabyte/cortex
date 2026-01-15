// Package parser provides tree-sitter based code parsing for multiple languages.
package parser

import "fmt"

// ParseError represents a parsing error with location information.
type ParseError struct {
	Message string
	File    string
	Line    uint32
	Column  uint32
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.File != "" {
		return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message)
	}
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Column, e.Message)
}

// UnsupportedLanguageError is returned when attempting to parse an unsupported language.
type UnsupportedLanguageError struct {
	Language string
}

// Error implements the error interface.
func (e *UnsupportedLanguageError) Error() string {
	return fmt.Sprintf("unsupported language: %s", e.Language)
}

// FileReadError is returned when a file cannot be read.
type FileReadError struct {
	Path string
	Err  error
}

// Error implements the error interface.
func (e *FileReadError) Error() string {
	return fmt.Sprintf("failed to read file %s: %v", e.Path, e.Err)
}

// Unwrap returns the underlying error.
func (e *FileReadError) Unwrap() error {
	return e.Err
}
