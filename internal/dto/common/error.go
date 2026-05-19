package common

import "net/http"


const (
	ErrCodeBadRequest            = "BAD_REQUEST"
	ErrCodeValidation            = "VALIDATION_ERROR"
	ErrCodeInvalidInput          = "INVALID_INPUT"
	ErrCodeInvalidBody           = "INVALID_REQUEST_BODY"
	ErrCodeInvalidQuery          = "INVALID_QUERY_PARAMETERS"
	ErrCodeNotFound              = "RESOURCE_NOT_FOUND"
	ErrCodeTicketNotFound        = "TICKET_NOT_FOUND"
	ErrCodeUnauthorized          = "UNAUTHORIZED"
	ErrCodeForbidden             = "FORBIDDEN"
	ErrCodeConflict              = "CONFLICT"
	ErrCodeInvalidTransition     = "INVALID_STATUS_TRANSITION"
	ErrCodeInvalidFlow           = "INVALID_FLOW_TICKET"
	ErrCodeEmptyBody             = "EMPTY_REQUEST_BODY"
	ErrCodeEmptyBatch            = "EMPTY_BATCH"
	ErrCodeBatchTooLarge         = "BATCH_TOO_LARGE"
	ErrCodeUnsupportedFileFormat = "UNSUPPORTED_FILE_FORMAT"
	ErrCodeInternal              = "INTERNAL_SERVER_ERROR"
)


type Error struct {
	Code    string        `json:"code"`
	Status  int           `json:"status"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}
type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}


func NewBadRequest(code, message string, details ...ErrorDetail) *Error {
	return &Error{
		Code:    code,
		Status:  http.StatusBadRequest,
		Message: message,
		Details: details,
	}
}

func NewValidation(message string, details []ErrorDetail) *Error {
	return &Error{
		Code:    ErrCodeValidation,
		Status:  http.StatusBadRequest,
		Message: message,
		Details: details,
	}
}

func NewNotFound(code, message string) *Error {
	return &Error{
		Code:    code,
		Status:  http.StatusNotFound,
		Message: message,
	}
}

func NewUnauthorized(code, message string) *Error {
	return &Error{
		Code:    code,
		Status:  http.StatusUnauthorized,
		Message: message,
	}
}

func NewForbidden(code, message string) *Error {
	return &Error{
		Code:    code,
		Status:  http.StatusForbidden,
		Message: message,
	}
}

func NewConflict(code, message string) *Error {
	return &Error{
		Code:    code,
		Status:  http.StatusConflict,
		Message: message,
	}
}

func NewInternal(message string) *Error {
	return &Error{
		Code:    ErrCodeInternal,
		Status:  http.StatusInternalServerError,
		Message: message,
	}
}
