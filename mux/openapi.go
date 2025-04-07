package mux

import (
	"fmt"

	"github.com/MarceloPetrucio/go-scalar-api-reference"

	"github.com/obadmatar/base/log"
)

// ApiDocsHandler serves the API documentation in HTML format.
// It uses the `go-scalar-api-reference` package to generate HTML content for the API documentation.
func ApiDocsHandler(specURL, pageTitle string) HandlerFunc {
	return func(ctx *Context) error {
		// Generate HTML
		htmlContent, err := scalar.ApiReferenceHTML(&scalar.Options{

			DarkMode: true,
			Layout:   "classic",
			Theme:    "alternate",

			HideModels:         false,
			ShowSidebar:        true,
			HideDownloadButton: true,

			SpecURL:       specURL,
			CustomOptions: scalar.CustomOptions{PageTitle: pageTitle},
		})

		if err != nil {
			log.Error("openapi: ", "error", err)
			return ctx.InternalServerError(M{"error": err})
		}

		_, err = fmt.Fprintln(ctx.rsp, htmlContent)
		if err != nil {
			log.Error("openapi: ", "error", err)
			return ctx.InternalServerError(M{"error": err})
		}

		return nil
	}
}
