package mux

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/cors"

	"github.com/obadmatar/base/log"
	"github.com/obadmatar/base/valid"
)

type Config struct {
	// Port specifies the port on which the HTTP server listens (default: "8080").
	Port string `env:"HTTP_PORT" default:"8080"`

	// ReadTimeout is the maximum duration in seconds for reading the request
	// before timing out.
	ReadTimeout int `env:"HTTP_READ_TIMEOUT"`

	// WriteTimeout is the maximum duration in seconds for writing the response
	// before timing out.
	WriteTimeout int `env:"HTTP_WRITE_TIMEOUT"`

	// IdleTimeout defines the maximum duration in seconds a connection can stay
	// idle before being closed.
	IdleTimeout int `env:"HTTP_IDLE_TIMEOUT"`

	// MaxHeaderBytes specifies the maximum size in bytes of request headers.
	MaxHeaderBytes int `env:"HTTP_MAX_HEADER_BYTES"`

	// GracefulShutdown is the timeout in seconds to allow active connections
	// to close before the server shuts down.
	GracefulShutdown int `env:"GRACEFUL_SHUTDOWN_TIMEOUT" default:"10"`

	// AllowedOrigins is a list of origins a cross-domain request can be executed from.
	// If the special "*" value is present in the list, all origins will be allowed.
	// An origin may contain a wildcard (*) to replace 0 or more characters
	// (i.e.: http://*.domain.com). Usage of wildcards implies a small performance penalty.
	// Only one wildcard can be used per origin.
	// Default value is ["*"]
	AllowedOrigins []string `env:"ALLOWED_ORIGINS" default:"*"`
}

// Validate ensures that the Config struct has valid values.
func (c *Config) Validate() error {
	// Port validation
	if c.Port == "" {
		log.Warn("Port is empty, defaulting to 8080")
		c.Port = "8080"
	}

	if !isValidPort(c.Port) {
		log.Warn("Invalid port %s, using default value 8080", c.Port)
		c.Port = "8080"
	}

	// Timeout validations
	if c.ReadTimeout < 0 {
		log.Warn("ReadTimeout is too low, defaulting to 0")
		c.ReadTimeout = 0
	}

	if c.WriteTimeout < 0 {
		log.Warn("WriteTimeout is too low, defaulting to 0")
		c.WriteTimeout = 0
	}

	if c.IdleTimeout < 0 {
		log.Warn("IdleTimeout is too low, defaulting to 0")
		c.IdleTimeout = 0
	}

	// Graceful shutdown validation
	if c.GracefulShutdown < 0 {
		log.Warn("GracefulShutdown timeout is too low, defaulting to 10")
		c.GracefulShutdown = 10
	}

	// MaxHeaderBytes validation
	if c.MaxHeaderBytes <= 0 {
		log.Warn("MaxHeaderBytes is too low, defaulting to 1048576")
		c.MaxHeaderBytes = 1048576 // 1MB
	}

	// Final validation check for non-negative timeout values
	if c.ReadTimeout < 0 {
		log.Error("Invalid ReadTimeout, must be non-negative", "value", c.ReadTimeout)
		return errors.New("ReadTimeout cannot be negative")
	}

	if c.WriteTimeout < 0 {
		log.Error("Invalid WriteTimeout, must be non-negative", "value", c.WriteTimeout)
		return errors.New("WriteTimeout cannot be negative")
	}

	if c.IdleTimeout < 0 {
		log.Error("Invalid IdleTimeout, must be non-negative", "value", c.IdleTimeout)
		return errors.New("IdleTimeout cannot be negative")
	}

	if c.GracefulShutdown < 0 {
		log.Error("Invalid GracefulShutdown, must be non-negative", "value", c.GracefulShutdown)
		return errors.New("GracefulShutdown timeout cannot be negative")
	}

	if c.MaxHeaderBytes <= 0 {
		log.Error("Invalid MaxHeaderBytes, must be positive", "value", c.MaxHeaderBytes)
		return errors.New("MaxHeaderBytes must be positive")
	}

	return nil
}

// isValidPort checks if the given string is a valid port number
func isValidPort(port string) bool {
	// Check if the port string is a valid integer and within the range of 1-65535
	if strings.HasPrefix(port, ":") {
		port = port[1:] // strip the colon if present
	}
	portInt, err := strconv.Atoi(port)
	return err == nil && portInt > 0 && portInt <= 65535
}

// Handler defines an interface for handling HTTP requests.
// Custom handlers must implement this interface.
// Handle receives a Context and returns an error if the processing fails.
type Handler interface {
	Handle(ctx *Context) error
}

// MiddlewareFunc defines a function to process middleware.
// Middleware wraps a Handler to provide additional processing.
type MiddlewareFunc func(Handler) Handler

// HandlerFunc is an adapter to use ordinary functions as HTTP handlers.
type HandlerFunc func(ctx *Context) error

// Handle implements the Handler interface for HandlerFunc.
// It simply calls the underlying function.
func (f HandlerFunc) Handle(ctx *Context) error {
	return f(ctx)
}

// Router provides basic request routing and middleware support.
// It simplifies handler management compared to the default http.ServeMux.
type Router interface {
	// Handle registers a new route with a matcher for the URL path.
	// It maps the given pattern to the given Handler.
	Handle(pattern string, h Handler)

	// Use adds one or more middleware functions to the router.
	// Middleware is applied to all routes.
	Use(middleware ...MiddlewareFunc)

	// ListenAndServe starts the HTTP server on the configured address.
	ListenAndServe() error
}

type router struct {
	config   *Config
	mux      *http.ServeMux
	mwares   []MiddlewareFunc
	handlers map[string]Handler
}

// NewRouter creates a new Router with the provided logger.
func NewRouter(config *Config) Router {
	return &router{
		config:   config,
		mux:      http.NewServeMux(),
		mwares:   make([]MiddlewareFunc, 0),
		handlers: make(map[string]Handler),
	}
}

// Handle registers a new handler for the given pattern.
// Logs a warning if a handler for the pattern already exists.
func (r *router) Handle(pattern string, h Handler) {
	if _, found := r.handlers[pattern]; found {
		log.Fatal("mux: Handler already exists", "pattern", pattern)
	}
	r.handlers[pattern] = h
}

// Use adds middleware functions to the router.
func (r *router) Use(middleware ...MiddlewareFunc) {
	r.mwares = append(r.mwares, middleware...)
}

// applyMiddlewares wraps a handler with all registered middleware.
func (r *router) applyMiddlewares(h Handler) Handler {
	for i := len(r.mwares) - 1; i >= 0; i-- {
		h = r.mwares[i](h)
	}
	return h
}

// httpHandler adapts a custom Handler to a http.Handler.
func (r *router) httpHandler(h Handler) http.Handler {
	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		r.handleRequest(newContext(rsp, req), h)
	})
}

// ErrorResponse represents a standardized error response format for HTTP errors.
// It is used to provide consistent error details for validation errors, decoding issues,
// and internal server errors.
type ErrorResponse struct {
	Status  int               `json:"status"`  // HTTP status code
	Error   string            `json:"error"`   // "VALIDATION_ERROR", "DECODE_ERROR"..etc
	Message string            `json:"message"` // A user-friendly message describing the error
	Errors  map[string]string `json:"errors"`  // Field-specific friendly error message
}

// handleRequest centralizes request processing and error handling.
func (r *router) handleRequest(ctx *Context, h Handler) {
	defer func() {
		if rec := recover(); rec != nil {
			buf := make([]byte, 64<<10)           // 64KB
			buf = buf[:runtime.Stack(buf, false)] // Capture stack trace

			// Log the error and stack trace
			err := fmt.Sprintf("panic: %v\n%s", rec, string(buf))
			log.Error("mux: Panic in request handler", "method", ctx.Method(), "url", ctx.URI(), "error", err)

			// respond
			ctx.internalServerError()
		}
	}()

	// handles specific error types by sending appropriate responses.
	// If binding, validation or domain error, it responds accordingly
	// otherwise, it returns a 500 error.
	if err := h.Handle(ctx); err != nil {
		log.Error("mux: Error in handler", "method", ctx.Method(), "url", ctx.URI(), "error", err)
		// Handle Binding Errors
		var b *BindingError
		if errors.As(err, &b) {
			sendDecodeErrorResponse(ctx, b)
			return
		}

		// Handle Validation Errors
		var v valid.Errors
		if errors.As(err, &v) {
			sendValidationErrorResponse(ctx, v)
			return
		}

		// Handle Domain Not Found Errors
		var n *NotFoundError
		if errors.As(err, &n) {
			sendNotFoundErrorResponse(ctx, n)
			return
		}

		// Handle Domain Errors
		var d *DomainError
		if errors.As(err, &d) {
			sendDomainErrorResponse(ctx, d)
			return
		}

		// Return a generic 500 Internal Server Error for other errors
		ctx.internalServerError()

		// Un-handled error
		log.Error("mux: Error handling request", "url", ctx.URI(), "error", err)
	}
}

// ListenAndServe starts the HTTP server with the registered routes and handlers.
// It listens on the configured address and blocks until the server shuts down or encounters an error.
// Any server errors during shutdown are logged.
func (r *router) ListenAndServe() error {
	// Register routes with middleware applied.
	for pattern, handler := range r.handlers {
		// Apply any defined middlewares to the handlers.
		r.mux.Handle(pattern, r.httpHandler(r.applyMiddlewares(handler)))
	}

	// Needs to be updated to read host from config variables.
	addr := ":" + r.config.Port

	// CORS configurations
	opts := cors.Options{
		AllowedHeaders: []string{"*"},
		AllowedOrigins: r.config.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
	}

	// Apply CORS
	muxWithCORS := cors.New(opts).Handler(r.mux)

	// Configure the HTTP server with the given address and router.
	server := &http.Server{
		Addr:           addr,
		Handler:        muxWithCORS,
		MaxHeaderBytes: r.config.MaxHeaderBytes,
		IdleTimeout:    time.Duration(r.config.IdleTimeout) * time.Second,
		ReadTimeout:    time.Duration(r.config.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(r.config.WriteTimeout) * time.Second,
	}

	// Channel to capture server errors.
	done := make(chan error, 1)

	go func() {
		log.Info("mux: Starting HTTP server", "address", addr)
		// Listen for incoming HTTP requests; report any startup errors.
		done <- server.ListenAndServe()
	}()

	// Capture OS interrupt signals (SIGINT, SIGTERM).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-done:
		// Handle server errors during startup or runtime.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("mux: Server error occurred", "error", err)
			return err
		}
	case <-quit:
		// Handle graceful shutdown on receiving an interrupt signal.
		log.Info("mux: Shutdown signal received, shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.config.GracefulShutdown)*time.Second)
		defer cancel()

		// Attempt graceful shutdown with context.
		if err := server.Shutdown(ctx); err != nil {
			log.Error("mux: Error during server shutdown", "error", err)
			return err
		}
		log.Info("mux: Server gracefully stopped")
	}

	return nil
}
