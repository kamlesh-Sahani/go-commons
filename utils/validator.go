package utils

import (
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

var (
	// E.164 international phone number format: e.g. +14155552671 or +919876543210
	phoneRegex = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

	// Validate is the global validator instance with custom rules pre-registered
	Validate = validator.New()

	// Common disposable/temp email domains blacklist
	disposableEmailDomains = map[string]bool{
		"mailinator.com":    true,
		"10minutemail.com":  true,
		"guerrillamail.com": true,
		"tempmail.com":      true,
		"throwawaymail.com": true,
		"yopmail.com":       true,
		"sharklasers.com":   true,
		"trashmail.com":     true,
	}
)

func init() {
	notBlankValidator := func(fl validator.FieldLevel) bool {
		return strings.TrimSpace(fl.Field().String()) != ""
	}

	// Register "notblank" on Gin's internal binding validator engine
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("notblank", notBlankValidator)
	}

	// Register "notblank" on the standalone Validate instance
	_ = Validate.RegisterValidation("notblank", notBlankValidator)
}

// FormatValidationError formats raw validator.ValidationErrors into clean, user-friendly messages.
func FormatValidationError(err error) string {
	if err == nil {
		return ""
	}

	// Safely assert the error type to avoid panics
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok || len(validationErrors) == 0 {
		return "Invalid request payload"
	}

	// Grab only the first error for concise UX
	firstErr := validationErrors[0]
	field := strings.ToLower(firstErr.Field())

	switch firstErr.Tag() {
	case "required":
		return field + " is required"
	case "notblank":
		return field + " cannot be empty or whitespace only"
	case "email":
		return field + " must be a valid email address"
	case "min":
		return field + " must be at least " + firstErr.Param() + " characters"
	case "max":
		return field + " must be a maximum of " + firstErr.Param()
	case "len":
		return field + " must be exactly " + firstErr.Param()
	case "eq":
		return field + " must be equal to " + firstErr.Param()
	case "ne":
		return field + " must not be equal to " + firstErr.Param()
	case "gt":
		return field + " must be greater than " + firstErr.Param()
	case "gte":
		return field + " must be greater than or equal to " + firstErr.Param()
	case "lt":
		return field + " must be less than " + firstErr.Param()
	case "lte":
		return field + " must be less than or equal to " + firstErr.Param()
	case "url":
		return field + " must be a valid URL"
	case "uuid":
		return field + " must be a valid UUID"
	case "alpha":
		return field + " can only contain alphabetic characters"
	case "alphanum":
		return field + " can only contain alphanumeric characters"
	case "numeric":
		return field + " must be a valid numeric value"
	case "oneof":
		return field + " must be one of: " + firstErr.Param()
	default:
		return field + " is invalid"
	}
}

// IsValidEmail verifies if the provided string is a valid email address using standard RFC formatting.
func IsValidEmail(email string) bool {
	_, err := mail.ParseAddress(strings.TrimSpace(email))
	return err == nil
}

// IsDisposableEmail checks whether the email domain belongs to known disposable/temporary services.
func IsDisposableEmail(email string) bool {
	parts := strings.Split(strings.TrimSpace(strings.ToLower(email)), "@")
	if len(parts) != 2 {
		return false
	}
	return disposableEmailDomains[parts[1]]
}

// IsValidDate checks if the provided string is a valid date.
// It supports full ISO-8601/RFC3339 timestamps (e.g., "2005-04-07T10:00:00.000Z")
// as well as simple calendar dates (e.g., "2005-04-07").
func IsValidDate(dateStr string) bool {
	if dateStr == "" {
		return false
	}

	// 1. Check standard ISO/RFC3339 timestamp (e.g. from dayjs.toISOString())
	if _, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return true
	}

	// 2. Check simple calendar date (YYYY-MM-DD)
	if _, err := time.Parse(time.DateOnly, dateStr); err == nil {
		return true
	}

	return false
}

// IsValidPhone validates if a phone number conforms to the E.164 international standard.
func IsValidPhone(phone string) bool {
	phone = strings.TrimSpace(phone)
	return phoneRegex.MatchString(phone)
}

// IsValidUUID validates if a string is a valid UUID (v1-v5).
func IsValidUUID(id string) bool {
	_, err := uuid.Parse(strings.TrimSpace(id))
	return err == nil
}

// IsValidURL verifies if a string is an absolute HTTP/HTTPS URL.
func IsValidURL(rawURL string) bool {
	u, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
