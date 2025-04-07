package mux

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-viper/mapstructure/v2"

	"github.com/obadmatar/base/log"
)

type M map[string]any

// BindingError represents errors related to JSON body or URL Query Params bindings.
type BindingError struct {
	Message string
	Errors  map[string]string
}

// Error implements builtin.error interface
func (e *BindingError) Error() string {
	return e.Message
}

func newBindingError(format string, a ...any) *BindingError {
	return &BindingError{Message: fmt.Sprintf(format, a...)}
}

// encode writes data to the http response as JSON-encoded
// and sets the Content-Type header to "application/json"
func encode(w http.ResponseWriter, status int, body any, headers http.Header) error {
	// encode body to json
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	// add headers
	for h, v := range headers {
		w.Header()[h] = v
	}

	// set response status and content-type header
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(b)

	return nil
}

// decode parse JSON-encoded request body and store it in v
// it returns error if unknown fields found, body limit exceeded 1MB
// or body contains invalid JSON syntax, invalid JSON type or invalid field type
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	// limit request body to 1MB.
	maxBytes := 1_048_576
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	// init JSON decoder
	decoder := json.NewDecoder(r.Body)

	// only fields defined in v
	decoder.DisallowUnknownFields()

	// decode body input and store it in v
	err := decoder.Decode(v)
	if err == nil {
		// check if body contains only one single JSON value
		err = decoder.Decode(&struct{}{})
		if err != io.EOF {
			return newBindingError("body must only contain a single JSON value")
		}

		return nil
	}

	var syntaxError *json.SyntaxError
	var maxBytesError *http.MaxBytesError
	var unmarshalTypeError *json.UnmarshalTypeError
	var invalidUnmarshalError *json.InvalidUnmarshalError

	// check if it is invalid destination
	if errors.As(err, &invalidUnmarshalError) {
		panic(err)
	}

	// check if it is empty body error
	if errors.Is(err, io.EOF) {
		return newBindingError("body must be valid JSON")
	}

	// check if it is unexpected syntax errors in the JSON
	// open issue: https://github.com/golang/go/issues/25956
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return newBindingError("body contains badly-formed JSON")
	}

	// check if it is body size error
	if errors.As(err, &maxBytesError) {
		return newBindingError("body must not exceed %d bytes", maxBytesError.Limit)
	}

	// check if it is unknown field error
	if strings.HasPrefix(err.Error(), "json: unknown field ") {
		fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
		return newBindingError("body contains unknown keys %v", fieldName)
	}

	// check if it is invalid syntax error
	if errors.As(err, &syntaxError) {
		return newBindingError("body contains badly-formed JSON and can not be parsed")
	}

	// check if it is invalid type error
	if errors.As(err, &unmarshalTypeError) {
		if unmarshalTypeError.Field != "" {
			return newBindingError("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
		}

		return newBindingError("body contains incorrect JSON value that was not appropriate for the request body (at character %d)", unmarshalTypeError.Offset)
	}

	return err
}

// decodeURL is a helper function that processes the request query parameters.
func decodeURL(r *http.Request, v any) error {
	// Parse URL query parameters
	query := r.URL.Query()
	params := make(map[string]any)

	for key, values := range query {
		if len(values) == 1 {
			params[key] = values[0]
		} else {
			params[key] = values
		}
	}

	// Decode into the given struct
	decoderConfig := &mapstructure.DecoderConfig{
		Result:           v,
		Metadata:         nil,
		TagName:          "query",
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		return &BindingError{Message: err.Error()}
	}

	if err := decoder.Decode(params); err != nil {
		prefix := "decoding failed due to the following error(s):\n\n"
		fError := mapstructFieldErrors(strings.Replace(err.Error(), prefix, "", -1))
		return &BindingError{Message: "Query Params Decoding Failed", Errors: fError}
	}

	return nil
}

func mapstructFieldErrors(fieldError string) map[string]string {
	m := make(map[string]string)
	fieldErrors := strings.Split(fieldError, "\n")
	for _, fieldError := range fieldErrors {
		field, message := extractAndRemove(fieldError)
		m[field] = message
	}
	return m
}

// Function to extract the value between the first single quotes and return the modified string
func extractAndRemove(input string) (string, string) {
	// Regular expression to capture the value between the first set of single quotes
	re := regexp.MustCompile(`'([^']+)'`)

	// Find the first match (value between the first single quotes)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		// The value between the first single quotes
		value := matches[1]

		// Remove the value (including the single quotes) from the original string
		remaining := strings.TrimSpace(strings.Replace(input, "'"+value+"'", "", 1))

		return value, remaining
	}

	// Return empty string and input if no match found
	return "", input
}

func sendDecodeErrorResponse(ctx *Context, e *BindingError) {
	response := ErrorResponse{}
	response.Errors = e.Errors
	response.Message = e.Error()
	response.Error = "DECODE_ERROR"
	response.Status = http.StatusBadRequest
	if err := ctx.BadRequest(response); err != nil {
		log.Error("binding: failed to respond", "error", err)
		ctx.internalServerError()
	}
}
