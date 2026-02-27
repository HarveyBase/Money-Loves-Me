package errors

import "fmt"

// ErrorCode 表示交易系统的分类错误码。
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

// AppError 是所有模块使用的统一错误类型。
type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error
	Module  string
}

// Error 实现 error 接口。
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %d: %s: %v", e.Module, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %d: %s", e.Module, e.Code, e.Message)
}

// Unwrap 返回底层原因错误，以支持 errors.Is / errors.As。
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError 创建一个新的 AppError。
func NewAppError(code ErrorCode, message, module string, cause error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Module:  module,
		Cause:   cause,
	}
}

// Is 报告目标错误是否具有相同的错误码。
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
