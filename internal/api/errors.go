package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

// Error codes the CLI prints on the `code:` line.
const (
	CodeUnauthenticated = "unauthenticated"
	CodeForbidden       = "forbidden"
	CodeNotFound        = "not_found"
	CodeInvalid         = "invalid"
	CodeRateLimited     = "rate_limited"
	CodeServer          = "server_error"
	CodeHTTP            = "http_error"
	CodeNetwork         = "network_error"
)

// Error is a non-2xx answer from the API.
type Error struct {
	Status  int
	Code    string
	Message string
	// Fields holds the 422 messages by field name, such as "plan" or "url".
	Fields map[string][]string
}

func (e *Error) Error() string { return e.Message }

// FieldNames returns the field names of a 422 in a stable order.
func (e *Error) FieldNames() []string {
	names := make([]string, 0, len(e.Fields))
	for name := range e.Fields {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// NetworkError means the request never got an answer.
type NetworkError struct{ Err error }

func (e *NetworkError) Error() string { return e.Err.Error() }

func newError(status int, raw []byte) *Error {
	var body struct {
		Message string              `json:"message"`
		Errors  map[string][]string `json:"errors"`
	}
	// A body that is not JSON, such as a proxy's error page, leaves the fields empty.
	_ = json.Unmarshal(raw, &body)

	message := body.Message
	if message == "" {
		message = fmt.Sprintf("%d %s", status, http.StatusText(status))
	}

	return &Error{
		Status:  status,
		Code:    codeForStatus(status),
		Message: message,
		Fields:  body.Errors,
	}
}

func codeForStatus(status int) string {
	switch {
	case status == http.StatusUnauthorized:
		return CodeUnauthenticated
	case status == http.StatusForbidden:
		return CodeForbidden
	case status == http.StatusNotFound:
		return CodeNotFound
	case status == http.StatusUnprocessableEntity:
		return CodeInvalid
	case status == http.StatusTooManyRequests:
		return CodeRateLimited
	case status >= 500:
		return CodeServer
	default:
		return CodeHTTP
	}
}
