package base

import (
	"fmt"
)

type DomainError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type NotFoundError struct {
	DomainError
}

func (err *DomainError) Error() string {
	return err.Message
}

func Errorf(format string, a ...any) error {
	return &DomainError{
		Message: fmt.Sprintf(format, a...),
	}
}

func NotFoundErrorf(format string, a ...any) error {
	return &NotFoundError{
		DomainError: DomainError{
			Message: fmt.Sprintf(format, a...),
		},
	}
}
