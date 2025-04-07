package mux

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/obadmatar/base"
	"github.com/obadmatar/base/log"
	"github.com/obadmatar/base/valid"
)

// Context implements the req.Context interface, providing access to
// HTTP request and response details, along with request ID and user info.
type Context struct {
	context.Context
	requestID   string
	currentUser string
	req         *http.Request
	rsp         http.ResponseWriter
}

// http.Request Methods

// URI returns the request URI.
func (ctx *Context) URI() string {
	return ctx.req.URL.RequestURI()
}

// Method returns the HTTP method of the request.
func (ctx *Context) Method() string {
	return ctx.req.Method
}

// Headers returns the headers of the request.
func (ctx *Context) Headers() http.Header {
	return ctx.req.Header
}

// Header returns the value of a specific header.
func (ctx *Context) Header(key string) string {
	return ctx.req.Header.Get(key)
}

// Cookies returns all cookies sent with the request.
func (ctx *Context) Cookies() []*http.Cookie {
	return ctx.req.Cookies()
}

// Cookie returns the named cookie provided in the request.
func (ctx *Context) Cookie(name string) (*http.Cookie, error) {
	return ctx.req.Cookie(name)
}

// PathValue returns the value for the named path or empty if none.
func (ctx *Context) PathValue(name string) string {
	return ctx.req.PathValue(name)
}

// PathInt returns the value for the named path component as an integer.
// It returns 0 if the value is missing or not a valid integer.
func (ctx *Context) PathInt(name string) int {
	value := ctx.PathValue(name)
	if value == "" {
		return 0
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return val
}

// PathID returns the value for the named path {id} as string.
func (ctx *Context) PathID() string {
	return ctx.PathValue("id")
}

// PathIntID returns the value for the named path ID variable as integer.
func (ctx *Context) PathIntID(name string) (int, error) {
	var err error
	id := ctx.PathInt(name)
	if id == 0 {
		err = base.Errorf("invalid integer %s: %s", name, ctx.PathValue(name))
	}
	return id, err
}

// Query returns the first value for the named component of the query as a string
func (ctx *Context) Query(name string) string {
	return strings.TrimSpace(ctx.req.URL.Query().Get(name))
}

// QueryInt returns the first value for the named component of the query as an integer.
// It returns 0 if the value is missing or not a valid integer.
func (ctx *Context) QueryInt(name string) int {
	value := ctx.Query(name)
	if value == "" {
		return 0
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return val
}

// QueryIntOrError returns the first value for the named component of the query as an integer.
// It returns 0 and an error if the value is missing or not a valid integer.
func (ctx *Context) QueryIntOrError(name string) (int, error) {
	value := ctx.Query(name)
	if value == "" {
		return 0, fmt.Errorf("query parameter %s is missing", name)
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("query parameter %s is not a valid integer: %v", name, err)
	}
	return val, nil
}

// QueryBool returns the boolean value of the named query parameter
func (ctx *Context) QueryBool(name string) bool {
	v, _ := ctx.QueryBoolOrError(name)
	return v
}

// QueryBoolOrError returns the boolean value of the named query parameter or an error if parsing fails
func (ctx *Context) QueryBoolOrError(name string) (bool, error) {
	val := ctx.Query(name)
	if val == "" {
		return false, fmt.Errorf("query parameter %s not found", name)
	}
	boolVal, err := strconv.ParseBool(val)
	if err != nil {
		return false, fmt.Errorf("query parameter %s is not a valid boolean", name)
	}
	return boolVal, nil
}

// QueryParams returns the map of query parameters.
func (ctx *Context) QueryParams() map[string][]string {
	return ctx.req.URL.Query()
}

// Body returns the request's body.
func (ctx *Context) Body() io.ReadCloser {
	return ctx.req.Body
}

func (ctx *Context) RemoteAddr() string {
	// Check X-Forwarded-For header
	if forwardedFor := ctx.Header("X-Forwarded-For"); forwardedFor != "" {
		if ip := extractFirstIP(forwardedFor); ip != "" {
			if port := ctx.Header("X-Forwarded-Port"); port != "" {
				return fmt.Sprintf("%s:%s", ip, port)
			}
		}
	}

	// Check X-Real-IP header
	if realIP := ctx.req.Header.Get("X-Real-IP"); realIP != "" {
		if port := ctx.Header("X-Forwarded-Port"); port != "" {
			return fmt.Sprintf("%s:%s", realIP, port)
		}
	}

	// Fallback to req.RemoteAddr
	return ctx.req.RemoteAddr
}

func extractFirstIP(forwardedFor string) string {
	for _, ip := range strings.Split(forwardedFor, ",") {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			return ip
		}
	}
	return ""
}

// FormValue returns the first value for the named component of the form data.
func (ctx *Context) FormValue(key string) string {
	return ctx.req.FormValue(key)
}

// ParseMultipartForm parses a request body as multipart/form-data.
func (ctx *Context) ParseMultipartForm(maxMemory int64) error {
	return ctx.req.ParseMultipartForm(maxMemory)
}

// http.ResponseWriter Methods

// SetCookie sets a cookie on the response.
func (ctx *Context) SetCookie(cookie *http.Cookie) {
	http.SetCookie(ctx.rsp, cookie)
}

// Write writes data to the response.
func (ctx *Context) Write(data []byte) (int, error) {
	return ctx.rsp.Write(data)
}

// WriteHeader sets the HTTP status code for the response.
func (ctx *Context) WriteHeader(statusCode int) {
	ctx.rsp.WriteHeader(statusCode)
}

// SetHeader sets a header field to a specific value.
func (ctx *Context) SetHeader(key, value string) {
	ctx.rsp.Header().Set(key, value)
}

// SetHeaders sets multiple header fields.
func (ctx *Context) SetHeaders(headers map[string]string) {
	for key, value := range headers {
		ctx.rsp.Header().Set(key, value)
	}
}

// Custom Response methods

// OK sends a 200 OK response
func (ctx *Context) OK(body any) error {
	return encode(ctx.rsp, http.StatusOK, body, nil)
}

// Created sends a 201 Created response
func (ctx *Context) Created(body any) error {
	return encode(ctx.rsp, http.StatusCreated, body, nil)
}

// NotFound sends a 404 Not Found response.
func (ctx *Context) NotFound(body any) error {
	return encode(ctx.rsp, http.StatusNotFound, body, nil)
}

// UnAuthorized sends a 401 Unauthorized response.
func (ctx *Context) UnAuthorized(body any) error {
	return encode(ctx.rsp, http.StatusUnauthorized, body, nil)
}

// BadRequest sends a 400 Bad Request response.
func (ctx *Context) BadRequest(body any) error {
	return encode(ctx.rsp, http.StatusBadRequest, body, nil)
}

// badRequest sends a 400 Bad Request response.
func (ctx *Context) badRequest(body any) {
	if err := ctx.BadRequest(body); err != nil {
		ctx.internalServerError()
	}
}

// InternalServerError sends a 500 Internal Server Error response.
func (ctx *Context) internalServerError() {
	response := ErrorResponse{}
	response.Error = "INTERNAL_ERROR"
	response.Message = "Something went wrong"
	response.Status = http.StatusInternalServerError
	if err := ctx.InternalServerError(response); err != nil {
		log.Error("mux: failed to send response", "error", err)
	}
}

// InternalServerError sends a 500 Internal Server Error response.
func (ctx *Context) InternalServerError(body any) error {
	return encode(ctx.rsp, http.StatusInternalServerError, body, nil)
}

// Redirect sends a 302 Found response to the given URL.
func (ctx *Context) Redirect(url string) {
	http.Redirect(ctx.rsp, ctx.req, url, http.StatusFound)
}

// Extended Methods

// Normalizer is an interface for types that require normalization
// before further processing, ensuring fields are adjusted as needed
// (e.g., converting to lowercase) before validation or other operations.
type Normalizer interface {
	Normalize(ctx *Context)
}

// Decode parses the JSON-encoded request body into v and validates it.
// It first decodes the body into v, checking for syntax errors, unknown fields,
// and mismatched field types. Then it validates the struct using the validator package.
// Returns an error if decoding or validation fails.
func (ctx *Context) Decode(v any) error {
	w, r := ctx.rsp, ctx.req

	// Decode JSON body into v
	if err := decode(w, r, v); err != nil {
		return err
	}

	// Normalize if applicable
	if normalizer, ok := v.(Normalizer); ok {
		normalizer.Normalize(ctx)
	}

	// Validate decoded struct
	if err := valid.Struct(v); err != nil {
		return err
	}

	return nil
}

// DecodeURL ...
func (ctx *Context) DecodeURL(v any) error {
	r := ctx.req

	// Decode query params into v
	if err := decodeURL(r, v); err != nil {
		return err
	}

	// Normalize if applicable
	if normalizer, ok := v.(Normalizer); ok {
		normalizer.Normalize(ctx)
	}

	// Validate decoded struct
	if err := valid.Struct(v); err != nil {
		return err
	}

	return nil
}

// RequestID returns the unique request ID.
func (ctx *Context) RequestID() string {
	return ctx.requestID
}

// CurrentUser returns the current user associated with the request.
func (ctx *Context) CurrentUser() string {
	return ctx.currentUser
}

// newContext creates a new Context with a unique request ID.
func newContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		rsp:       w,
		req:       r,
		Context:   r.Context(),
		requestID: uuid.NewString(),
	}
}
