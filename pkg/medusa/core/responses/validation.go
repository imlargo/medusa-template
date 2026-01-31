package responses

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ErrorValidation formatea errores de validator de forma consistente
func ErrorValidation(c *gin.Context, err error) {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		details := make(map[string][]string)
		
		for _, err := range validationErrors {
			field := err.Field()
			message := formatValidationMessage(err)
			details[field] = append(details[field], message)
		}
		
		WriteErrorResponse(c, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Request validation failed", details)
		return
	}
	
	// Si no es error de validación, es error de binding (JSON malformado)
	ErrorBadRequest(c, "Invalid request format")
}

func formatValidationMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "field is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters", err.Param())
	case "max":
		return fmt.Sprintf("must be at most %s characters", err.Param())
	case "gte":
		return fmt.Sprintf("must be greater than or equal to %s", err.Param())
	case "lte":
		return fmt.Sprintf("must be less than or equal to %s", err.Param())
	case "url":
		return "must be a valid URL"
	case "oneof":
		return fmt.Sprintf("must be one of: %s", err.Param())
	default:
		return fmt.Sprintf("validation failed on '%s' rule", err.Tag())
	}
}
