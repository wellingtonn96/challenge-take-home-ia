package apierror

import "net/http"

const (
	CodeInvalidRequest      = "invalid_request"
	CodeNotFound            = "not_found"
	CodeConflict            = "conflict"
	CodeMethodNotAllowed    = "method_not_allowed"
	CodeInternal            = "internal_error"
	CodeDatabaseUnavailable = "database_unavailable"
	CodeRAGUnavailable      = "rag_unavailable"
)

// Error is the public HTTP error body: {"error","message"}.
// Status is not serialized; handlers write it as the HTTP status.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"error"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func Invalid(message string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: CodeInvalidRequest, Message: message}
}

func NotFound(message string) *Error {
	return &Error{Status: http.StatusNotFound, Code: CodeNotFound, Message: message}
}

func Conflict(message string) *Error {
	return &Error{Status: http.StatusConflict, Code: CodeConflict, Message: message}
}

func MethodNotAllowed() *Error {
	return &Error{
		Status:  http.StatusMethodNotAllowed,
		Code:    CodeMethodNotAllowed,
		Message: "method not allowed",
	}
}

func Internal(message string) *Error {
	return &Error{Status: http.StatusInternalServerError, Code: CodeInternal, Message: message}
}

func Unavailable(message string) *Error {
	return &Error{
		Status:  http.StatusServiceUnavailable,
		Code:    CodeDatabaseUnavailable,
		Message: message,
	}
}

func BadGateway(code, message string) *Error {
	if code == "" {
		code = CodeRAGUnavailable
	}
	return &Error{Status: http.StatusBadGateway, Code: code, Message: message}
}
