package medusa

import (
	"github.com/imlargo/medusa/pkg/medusa/context"
)

func init() {
	// Initialize error constructor for context package
	context.SetErrorConstructor(errorConstructor{})
	context.SetErrorConverter(errorConverter{})
}

// errorConstructor implements context.ErrorConstructor
type errorConstructor struct{}

func (errorConstructor) Validation(message string, details interface{}) error {
	return ErrValidation(message, details)
}

func (errorConstructor) BadRequest(message string) error {
	return ErrBadRequest(message)
}

// errorConverter implements context.ErrorConverter
type errorConverter struct{}

func (errorConverter) ToError(err error) context.ConvertedError {
	appErr := ToError(err)
	return context.ConvertedError{
		Code:     appErr.Code,
		Message:  appErr.Message,
		Details:  appErr.Details,
		Internal: appErr.Internal,
	}
}
