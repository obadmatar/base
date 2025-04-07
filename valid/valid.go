package valid

import (
	"errors"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// fieldCache for caching struct field mappings
var fieldCache sync.Map

var validate *validator.Validate

type validationErrors = validator.ValidationErrors

type Errors struct {
	cacheKey string
	validator.ValidationErrors
}

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())
}

// Struct validates a struct using the validator package
func Struct(s interface{}) error {
	// Generate or retrieve the cache key based on struct
	key := cacheTypeFields(s)

	// Perform validation
	err := validate.Struct(s)
	if err == nil {
		// No validation errors, return nil
		return nil
	}

	// If validation errors exist, process them
	var vrr validationErrors
	if !errors.As(err, &vrr) {
		// Un-known error, return as is
		return err
	}

	// Return an Errors struct containing the cache key and validation errors
	return Errors{
		cacheKey:         key,
		ValidationErrors: vrr,
	}
}

func cacheTypeFields(s interface{}) string {
	t := reflect.TypeOf(s)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check if struct type is already cached
	cacheKey := structCacheKey(t)
	if _, found := fieldCache.Load(cacheKey); found {
		return cacheKey
	}

	// Build fields map
	fieldsMap := make(map[string]string)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := fieldTagValue(field)
		fieldsMap[field.Name] = value
	}

	// Cache the result
	fieldCache.Store(cacheKey, fieldsMap)

	return cacheKey
}

func ExtractFieldErrors(vrr Errors) map[string]string {
	errorMap := make(map[string]string)
	fieldMap := make(map[string]string)

	// Check if struct type is already cached
	if cached, found := fieldCache.Load(vrr.cacheKey); found {
		fieldMap = cached.(map[string]string)
	}

	// error messages based on validation tags
	for _, e := range vrr.ValidationErrors {
		var errorMsg string

		switch e.Tag() {
		case "required":
			errorMsg = "is required"
		case "email":
			errorMsg = "Please provide a valid "
		case "min":
			errorMsg = "must be at least " + e.Param() + " characters"
		case "max":
			errorMsg = "cannot be more than " + e.Param() + " characters"
		case "gte":
			errorMsg = "must be greater than or equal to " + e.Param()
		case "lte":
			errorMsg = "must be less than or equal to " + e.Param()
		case "len":
			errorMsg = "must be exactly " + e.Param() + " characters"
		case "uuid":
			errorMsg = "must be a valid UUID"
		case "alpha":
			errorMsg = "must contain only alphabetic characters"
		case "alphanum":
			errorMsg = "must contain only alphanumeric characters"
		case "numeric":
			errorMsg = "must be a numeric value"
		case "url":
			errorMsg = "must be a valid URL"
		case "ip":
			errorMsg = "must be a valid IP address"
		case "ipv4":
			errorMsg = "must be a valid IPv4 address"
		case "ipv6":
			errorMsg = "must be a valid IPv6 address"
		case "gt":
			errorMsg = "must be greater than " + e.Param()
		case "lt":
			errorMsg = "must be less than " + e.Param()
		case "datetime":
			errorMsg = "must be a valid datetime"
		case "oneof":
			errorMsg = "must be one of: [" + strings.Join(strings.Split(e.Param(), " "), ",") + "]"
		// Comparison-based tags
		case "eq", "eqfield":
			errorMsg = "must be equal to " + e.Param()
		case "gtfield":
			errorMsg = "must be greater than " + e.Param()
		case "ltfield":
			errorMsg = "must be less than " + e.Param()
		case "nefield":
			errorMsg = "must not be equal to " + e.Param()
		case "eqcsfield":
			errorMsg = "must be equal to the related field " + e.Param()
		case "gtcsfield":
			errorMsg = "must be greater than the related field " + e.Param()
		case "ltcsfield":
			errorMsg = "must be less than the related field " + e.Param()
		// Network-based tags
		case "cidr":
			errorMsg = "must be a valid CIDR address"
		case "cidrv4":
			errorMsg = "must be a valid CIDR IPv4 address"
		case "cidrv6":
			errorMsg = "must be a valid CIDR IPv6 address"
		case "hostname":
			errorMsg = "must be a valid hostname"
		case "hostname_port":
			errorMsg = "must be a valid Host:Port"
		case "ip4_addr":
			errorMsg = "must be a valid IPv4 address"
		case "ip6_addr":
			errorMsg = "must be a valid IPv6 address"
		case "mac":
			errorMsg = "must be a valid MAC address"
		// String-based tags
		case "alphaunicode":
			errorMsg = "must contain only unicode alphabetic characters"
		case "alphanumunicode":
			errorMsg = "must contain only unicode alphanumeric characters"
		case "ascii":
			errorMsg = "must contain only ASCII characters"
		case "contains":
			errorMsg = "must contain the specified characters"
		case "containsany":
			errorMsg = "must contain any of the specified characters"
		case "lowercase":
			errorMsg = "must be lowercase"
		case "uppercase":
			errorMsg = "must be uppercase"
		// Format-based tags
		case "base64":
			errorMsg = "must be a valid Base64 encoded string"
		case "uuid3", "uuid4", "uuid5":
			errorMsg = "must be a valid UUID v3, v4, or v5"
		case "json":
			errorMsg = "must be a valid JSON string"
		case "credit_card":
			errorMsg = "must be a valid credit card number"
		// Other tags
		case "dir":
			errorMsg = "must be an existing directory"
		case "file":
			errorMsg = "must be an existing file"
		case "image":
			errorMsg = "must be a valid image file"
		case "unique":
			errorMsg = "must be unique"
		default:
			errorMsg = "is invalid"
		}

		// Get the field name based on available tag
		fieldName, exists := fieldMap[e.Field()]
		if !exists {
			// Fallback to lowercase field name if not found
			fieldName = strings.ToLower(e.Field())
		}

		errorMap[fieldName] = errorMsg
	}
	return errorMap
}

// fieldTagValue returns the appropriate tag value (json, query, or field name) based on the tag availability.
func fieldTagValue(field reflect.StructField) string {
	// tag: json
	if value := field.Tag.Get("json"); value != "" && value != "-" {
		return strings.Split(value, ",")[0]
	}
	// tag: query
	if value := field.Tag.Get("query"); value != "" && value != "-" {
		return strings.Split(value, ",")[0]
	}

	// Fallback to the field name
	return strings.ToLower(field.Name)
}

// structCacheKey
func structCacheKey(t reflect.Type) string {
	return t.String()
}
