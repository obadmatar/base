package mux

import (
	"net/http"

	"github.com/obadmatar/base"
	"github.com/obadmatar/base/log"
)

type DomainError = base.DomainError

type NotFoundError = base.NotFoundError

// sendDomainErrorResponse handles domain errors by sending a BadRequest response.
func sendDomainErrorResponse(ctx *Context, d *DomainError) {
	response := ErrorResponse{}
	response.Error = "DOMAIN_ERROR"
	response.Message = d.Message
	response.Status = http.StatusBadRequest
	if err := ctx.BadRequest(response); err != nil {
		log.Error("mux: failed to respond", "error", err)
		ctx.internalServerError()
	}
}

// sendNotFoundErrorResponse handles domain errors by sending a BadRequest response.
func sendNotFoundErrorResponse(ctx *Context, d *NotFoundError) {
	response := ErrorResponse{}
	response.Error = "DOMAIN_ERROR"
	response.Message = d.Message
	response.Status = http.StatusNotFound
	if err := ctx.NotFound(response); err != nil {
		log.Error("mux: failed to respond", "error", err)
		ctx.internalServerError()
	}
}
