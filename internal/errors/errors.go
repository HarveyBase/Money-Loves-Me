package errors

import "fmt"

// ErrorCode represents categorized error codes for the trading system.
type ErrorCode int

const (
	ErrNetwork     ErrorCode = 1000
	ErrAuth        ErrorCode = 2000
	ErrValidation  ErrorCode = 3000
	ErrRiskControl ErrorCode = 4000
	ErrDatabase    ErrorCode = 5000
	ErrConfig      ErrorCode = 6000
	ErrBinanceAPI  ErrorCode = 7000
)

// AppError is the unified error type used across all modules.
type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Module  string
}

// Error satisfies the error interface.
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %d: %s: %v", e.Module, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %d: %s", e.Module, e.Code, e.Message)
}

// Unwrap returns the underlying cause for errors.Is / errors.As support.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError creates a new AppError.
func NewAppError(code ErrorCode, message, module string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Module:  module,
		Cause:   cause,
	}
}

// Is reports whether the target error has the same error code.
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
