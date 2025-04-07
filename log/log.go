package log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
)

// Level defines log levels.x
type Level = zerolog.Level

var defaultLogger = NewLogger(&Config{Level: "INFO", Format: "text"})

const (
	// DebugLevel defines debug log level.
	DebugLevel Level = iota
	// InfoLevel defines info log level.
	InfoLevel
	// WarnLevel defines warn log level.
	WarnLevel
	// ErrorLevel defines error log level.
	ErrorLevel
	// FatalLevel defines fatal log level.
	FatalLevel
	// PanicLevel defines panic log level.
	PanicLevel
	// NoLevel defines an absent log level.
	NoLevel
	// Disabled disables the logger.
	Disabled

	// TraceLevel defines trace log level.
	TraceLevel Level = -1
)

// Config holds the configuration parameters for logging.
type Config struct {
	// LogLevel defines the minimum log severity level to output.
	// Available values: "DEBUG", "INFO", "WARN", "ERROR", "FATAL" (default: "INFO").
	Level string `env:"LOG_LEVEL" default:"INFO"`

	// LogFormat specifies the format of the logs.
	// Available values: "json" or "text" (default: "text").
	Format string `env:"LOG_FORMAT" default:"json"`

	// WithCaller specifies whether to include the caller information in the log output.
	// Default is false (caller information is not included).
	WithCaller bool `env:"LOG_CALLER" default:"false"`
}

func (c *Config) validate() error {
	if !isValidLogLevel(c.Level) {
		defaultLogger.Warn("config: Invalid LogLevel, defaulting to INFO", "current_value", c.Level)
		c.Level = "INFO"
	}
	if !isValidLogFormat(c.Format) {
		defaultLogger.Warn("config: Invalid LogFormat, defaulting to TEXT", "current_value", c.Format)
		c.Format = "text"
	}
	return nil
}

func (c *Config) level() Level {
	switch c.Level {
	case "TRACE":
		return TraceLevel
	case "DEBUG":
		return DebugLevel
	case "INFO":
		return InfoLevel
	case "WARN":
		return WarnLevel
	case "ERROR":
		return ErrorLevel
	case "FATAL":
		return FatalLevel
	case "PANIC":
		return PanicLevel
	case "NONE":
		return NoLevel
	case "DISABLED":
		return Disabled
	default:
		return InfoLevel
	}
}

func isValidLogLevel(level string) bool {
	validLevels := []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}
	for _, l := range validLevels {
		if strings.ToUpper(level) == l {
			return true
		}
	}
	return false
}

func isValidLogFormat(format string) bool {
	validFormats := []string{"json", "plain"}
	for _, f := range validFormats {
		if strings.ToLower(format) == f {
			return true
		}
	}
	return false
}

// Info logs general informational messages about application flow or user actions.
// Use for routine status updates or significant events during normal operations.
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

// InfoContext logs informational messages with additional context (e.g., request data).
// Ideal for tracking events tied to specific requests or sessions.
func InfoContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.InfoContext(ctx, msg, args...)
}

// Debug logs verbose messages intended for debugging and troubleshooting.
// Use to capture detailed internal states during development or issue investigation.
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

// DebugContext logs debug messages with context, useful for diagnosing issues with more details.
// Helps correlate debugging data to specific requests or operations.
func DebugContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.DebugContext(ctx, msg, args...)
}

// Error logs error messages for unexpected conditions requiring attention.
// Use when the application encounters issues that might disrupt normal operations.
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

// ErrorContext logs error messages with additional context to provide more insight.
// Useful for tracing the source of errors within a specific request or session.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.ErrorContext(ctx, msg, args...)
}

// Warn logs warning messages about potential issues that do not immediately impact functionality.
// Use when a non-critical condition could become a problem if not addressed.
func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

// WarnContext logs warnings with context, aiding in identifying non-critical issues in specific contexts.
// Helps track situations where further investigation is needed.
func WarnContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.WarnContext(ctx, msg, args...)
}

// Fatal logs critical errors that will likely lead to application termination.
// Use when a severe failure occurs, preventing the application from continuing.
func Fatal(msg string, args ...any) {
	defaultLogger.Fatal(msg, args...)
}

// FatalContext logs critical errors with context, signaling the need for immediate application shutdown.
// Used for fatal issues that require termination or recovery actions.
func FatalContext(ctx context.Context, msg string, args ...any) {
	defaultLogger.FatalContext(ctx, msg, args...)
}

// SetLevel sets the minimum log level.
// To turn off all logs, set level Disabled.
func SetLevel(level Level) {
	defaultLogger.SetLevel(level)
}

// SetDefaultLogger sets the internal default logger.
func SetDefaultLogger(l *Logger) {
	defaultLogger = l
	defaultLogger.skip = 2
}

// Logger defines methods for logging messages at various levels, supporting both standard and
// context-aware logging. It allows tracking application behavior with flexible logging options,
// whether for normal operation, debugging, error handling, or critical failures.
type Logger struct {
	skip    int
	handler zerolog.Logger

	// rightAlignPrefix controls whether the prefix (before the colon) in the log message should be right-aligned.
	rightAlignPrefix bool
}

// SetGlobalLevel sets the global override for log level. If this
// values is raised, all Loggers will use at least this value.
//
// To globally disable logs, set GlobalLevel Disabled.
func SetGlobalLevel(level Level) {
	zerolog.SetGlobalLevel(level)
}

// textDefaultPartsOrder return the order of parts in output.
func textDefaultPartsOrder(enableCaller bool) []string {
	parts := make([]string, 0)
	// with caller
	if enableCaller {
		parts = append(parts, zerolog.CallerFieldName)
	}

	parts = append(parts, zerolog.TimestampFieldName)
	parts = append(parts, zerolog.LevelFieldName)
	parts = append(parts, zerolog.MessageFieldName)
	return parts
}

// NewLogger creates a new logger based on the provided config
func NewLogger(c *Config) *Logger {
	var logger zerolog.Logger

	//  TimestampFieldName is the field name used for the logger timestamp field
	zerolog.TimestampFieldName = "log_timestamp"

	// options
	rightAlignPrefix := false

	// JSON Logger
	if c.Format == "json" {
		// Create JSON formatted logger
		logger = zerolog.New(os.Stdout).Level(c.level()).With().Timestamp().Logger()
	}

	// Default Console Logger
	if c.Format == "text" {
		// Enable prefix right alignment
		rightAlignPrefix = false

		// Handle Console Output (default: true)
		writer := zerolog.ConsoleWriter{Out: os.Stdout}
		writer.TimeFormat = time.DateTime
		writer.FormatCaller = fixedLengthCallerFormatter
		writer.PartsOrder = textDefaultPartsOrder(c.WithCaller)
		logger = zerolog.New(writer).Level(c.level()).With().Timestamp().Logger()
	}

	return &Logger{skip: 1, handler: logger, rightAlignPrefix: rightAlignPrefix}
}

func (l *Logger) SetLevel(level Level) {
	l.handler = l.handler.Level(level)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.handler.Debug().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
	l.handler.Debug().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) Info(msg string, args ...any) {
	l.handler.Info().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
	l.handler.Info().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) Warn(msg string, args ...any) {
	l.handler.Warn().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
	l.handler.Warn().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) Error(msg string, args ...any) {
	l.handler.Error().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) ErrorContext(ctx context.Context, msg string, args ...any) {
	l.handler.Error().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) Fatal(msg string, args ...any) {
	l.handler.Fatal().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

func (l *Logger) FatalContext(ctx context.Context, msg string, args ...any) {
	l.handler.Fatal().Fields(args).Caller(l.skip).Msg(l.withPrefixAlignment(msg))
}

// withPrefixAlignment aligns the prefix part of the log message to the right and appends the actual log message.
func (l *Logger) withPrefixAlignment(message string) string {
	if !l.rightAlignPrefix {
		return message
	}

	parts := strings.SplitN(message, ": ", 2)

	// If no prefix is found, return the original message
	if len(parts) < 2 {
		return message
	}

	prefix := parts[0]
	msg := parts[1]

	const prefixWidth = 11

	// If the prefix is shorter than the required width, pad it with spaces
	if len(prefix) < prefixWidth {
		var result strings.Builder
		result.Grow(prefixWidth + len(prefix) + 2 + len(msg)) // Pre-allocate space

		remainingSpaces := prefixWidth - len(prefix)
		// Add padding spaces
		for i := 0; i < remainingSpaces; i++ {
			result.WriteByte(' ')
		}
		result.WriteString(prefix + ": ")
		result.WriteString(msg)

		return result.String()
	}

	// Return the original message if the prefix is already wide enough
	return message
}

// fixedLengthCallerFormatter formats the caller with the package name and file name, left-aligned and colored.
func fixedLengthCallerFormatter(caller interface{}) string {
	// Convert the caller (which is an interface) to a string (which is the full file path)
	file, ok := caller.(string)
	if !ok {
		return ""
	}

	// Extract the file name (without the path)
	dir, fileName := filepath.Split(file)

	// Extract the package name (which is the last part of the directory path)
	packageName := filepath.Base(dir)

	// Combine package name and file name
	packageFileName := fmt.Sprintf("%s/%s", packageName, fileName)

	// Ensure the combined package and file name has a fixed length
	const fixedLength = 30
	if len(packageFileName) < fixedLength {
		// Pad with spaces to the right to make the length fixed (left-aligned)
		packageFileName = fmt.Sprintf("%-*s:", fixedLength, packageFileName)
	} else if len(packageFileName) > fixedLength {
		// Truncate the combined name if it's longer than the fixed length
		packageFileName = packageFileName[len(packageFileName)-fixedLength:]
	}

	// Color the caller with a custom color (blue in this case)
	coloredCaller := color.New(color.FgBlue).Sprintf("%s", packageFileName)

	return coloredCaller
}
