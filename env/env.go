package env

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"github.com/obadmatar/base/log"
)

// Validator is for types that implement custom validation logic.
type Validator interface {
	Validate() error
}

// Load reads environment variables from the specified config file(s).
// If no file paths are provided, it uses APP_ENV to determine the appropriate file:
//
// - APP_ENV="dev"   →  config/.env.dev,   Loads: config/.env
// - APP_ENV="prod"  →  config/.env.prod,  Loads: config/.env
// - APP_ENV="local" →  config/.env.local, Loads: config/.env
//
// Defaults to "local" if APP_ENV is unset or unrecognized.
// Parses the variables into the provided config struct and validates them if applicable.
func Load[T any](filePaths ...string) (*T, error) {
	var config T

	// Determine which config files to load (use APP_ENV-based defaults if no file is provided)
	files := getConfigFiles(filePaths)

	// Load environment variables from the config file(s)
	if err := loadEnvFiles(files); err != nil {
		log.Info("env: config from system environment variables")
	}

	// Parse the environment variables into the config struct
	if err := parseEnvVars(&config); err != nil {
		return nil, err
	}

	// Validate the config if it implements the Validator interface
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	// Log default values used
	checkAndLogDefaultValues(&config)

	return &config, nil
}

// getConfigFiles determines the config file paths based on APP_ENV.
// returns paths ["config/.env.local", "config/.env"] if no APP_ENV.
func getConfigFiles(filePaths []string) []string {
	// If file paths are provided, use them
	if len(filePaths) > 0 {
		return filePaths
	}

	// If no file paths are provided, get the environment variable APP_ENV
	appEnv := os.Getenv("APP_ENV")

	// Determine the config file based on APP_ENV
	switch appEnv {
	case "dev":
		return []string{"config/.env.dev", "config/.env"}
	case "prod":
		return []string{"config/.env.prod", "config/.env"}
	case "local":
		return []string{"config/.env.local", "config/.env"}
	default:
		// If APP_ENV is not set or has an unknown value, fall back to the default .env and .env.local
		log.Warn("APP_ENV not set, using 'local'. Options: dev, prod, local")
		return []string{"config/.env.local", "config/.env"}
	}
}

// loadEnvFiles loads environment variables from the specified configuration files in order.
// It attempts to load each file and logs warnings if any fail to load.
// The order in which files are provided determines the priority—later files do not override earlier ones.
func loadEnvFiles(files []string) error {
	var loadErrors []string

	// Try loading each file
	for _, file := range files {
		if err := godotenv.Load(file); err != nil {
			loadErrors = append(loadErrors, file)
			log.Warn("env: failed to load config file, skipping", "file", file)
		} else {
			log.Info("env: loaded environment variables from", "file", file)
		}
	}

	// If no files were successfully loaded, return an error indicating which files failed
	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load config files: %v", loadErrors)
	}

	return nil
}

// parseEnvVars parses environment variables into the provided config struct using caarlos0/env.
func parseEnvVars(config any) error {
	opts := env.Options{DefaultValueTagName: "default", RequiredIfNoDef: true}
	if err := env.ParseWithOptions(config, opts); err != nil {
		return formatEnvParseError(err)
	}
	return nil
}

// formatEnvParseError formats the error to log each missing environment variable
func formatEnvParseError(err error) error {
	// Split the error string into individual error variables
	errorString := err.Error()

	// format the error to split each variable error on a new line
	var envErrors []string
	for _, line := range strings.Split(errorString, ";") {
		line = strings.TrimSpace(line)
		if line != "" {
			// format and log env errors
			line = strings.Replace(line, "\"", "", -1)
			line = strings.Replace(line, "env: ", "", -1)
			log.Error("env: parsing failed", "error", line)
			envErrors = append(envErrors, line)
		}
	}

	// Return a general error for missing required environment variables
	if len(envErrors) > 0 {
		return fmt.Errorf("parsing failed check logs for missing or invalid environemnt variables")
	}

	return err
}

// validateConfig checks if the config implements the Validator interface and validates it.
func validateConfig[T any](config *T) error {
	if v, ok := any(config).(Validator); ok {
		if err := v.Validate(); err != nil {
			log.Error("env: config validation failed", "error", err)
			return err
		}
	}
	return nil
}

// Helper function to check and log if the default value is used for all fields in the struct
func checkAndLogDefaultValues[T any](config *T) {
	v := reflect.ValueOf(config).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		defaultValueTag := fieldType.Tag.Get("default")

		// Only check if a default value is provided
		if defaultValueTag != "" {
			// Compare the field value to its default value
			if fmt.Sprintf("%v", field.Interface()) == defaultValueTag {
				log.Warn("env: using default value for env", "name", fieldType.Tag.Get("env"), "value", field.Interface())
			}
		}
	}
}
