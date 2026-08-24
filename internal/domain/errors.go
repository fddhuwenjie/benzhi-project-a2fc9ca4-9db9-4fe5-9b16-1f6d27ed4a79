package domain

import "fmt"

type ErrorCode string

const (
	CodeValidation ErrorCode = "validation_failed"
	CodeNotFound   ErrorCode = "not_found"
	CodeConflict   ErrorCode = "conflict"
	CodeState      ErrorCode = "invalid_state"
	CodeForbidden  ErrorCode = "forbidden"
	CodeFrozen     ErrorCode = "case_frozen"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type DomainError struct {
	Code    ErrorCode    `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

func (e *DomainError) Error() string { return e.Message }

func ValidationError(fields ...FieldError) error {
	return &DomainError{Code: CodeValidation, Message: "输入内容未通过校验", Fields: fields}
}

func NewError(code ErrorCode, message string) error {
	return &DomainError{Code: code, Message: message}
}

func StateError(state CaseState, action string) error {
	return NewError(CodeState, fmt.Sprintf("案件状态 %s 不允许执行 %s", state, action))
}

func AsDomainError(err error) *DomainError {
	if value, ok := err.(*DomainError); ok {
		return value
	}
	return nil
}
