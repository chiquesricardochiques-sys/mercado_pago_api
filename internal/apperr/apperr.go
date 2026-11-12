package apperr

import (
	"errors"
	"net/http"
)

// Error é o erro padrão da API: já carrega o status HTTP e um código estável para os apps tratarem.
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"error"`
	Detail  string `json:"detail,omitempty"`
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return e.Message + ": " + e.Detail
	}
	return e.Message
}

func Validation(msg string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "validation_error", Message: msg}
}

func Conflict(code, msg string) *Error {
	return &Error{Status: http.StatusConflict, Code: code, Message: msg}
}

func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}