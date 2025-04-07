package mux

import (
	"net/http"

	"github.com/obadmatar/base/log"
	"github.com/obadmatar/base/valid"
)

// sendValidationErrorResponse handles validation errors by sending a BadRequest response
// with the error details, including the field names and corresponding error messages.
func sendValidationErrorResponse(ctx *Context, e valid.Errors) {
	response := ErrorResponse{}
	response.Error = "VALIDATION_ERROR"
	response.Message = "Invalid Request"
	response.Status = http.StatusBadRequest
	response.Errors = valid.ExtractFieldErrors(e)
	if err := ctx.BadRequest(response); err != nil {
		log.Error("validate: failed to respond", "error", err)
		ctx.internalServerError()
	}
}
