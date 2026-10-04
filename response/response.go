package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIResponse represents the standard JSON API response structure.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RespondJSON sends a JSON response with status, success flag, message, and data.
func RespondJSON(c *gin.Context, status int, success bool, message string, data interface{}) {
	if message == "" {
		if success {
			message = "Success"
		} else {
			message = "Something went wrong"
		}
	}
	c.JSON(status, APIResponse{
		Success: success,
		Message: message,
		Data:    data,
	})
}

// Success sends a 200 OK success response with data.
func Success(c *gin.Context, message string, data interface{}) {
	RespondJSON(c, http.StatusOK, true, message, data)
}

// Created sends a 201 Created response.
func Created(c *gin.Context, message string, data interface{}) {
	RespondJSON(c, http.StatusCreated, true, message, data)
}

// BadRequest sends a 400 Bad Request error response.
func BadRequest(c *gin.Context, message string) {
	RespondJSON(c, http.StatusBadRequest, false, message, nil)
}

// Unauthorized sends a 401 Unauthorized error response.
func Unauthorized(c *gin.Context, message string) {
	RespondJSON(c, http.StatusUnauthorized, false, message, nil)
}

// Forbidden sends a 403 Forbidden error response.
func Forbidden(c *gin.Context, message string) {
	RespondJSON(c, http.StatusForbidden, false, message, nil)
}

// NotFound sends a 404 Not Found error response.
func NotFound(c *gin.Context, message string) {
	RespondJSON(c, http.StatusNotFound, false, message, nil)
}

// Conflict sends a 409 Conflict error response.
func Conflict(c *gin.Context, message string) {
	RespondJSON(c, http.StatusConflict, false, message, nil)
}

// PaymentRequired sends a 402 Payment Required error response.
func PaymentRequired(c *gin.Context, message string) {
	RespondJSON(c, http.StatusPaymentRequired, false, message, nil)
}

// InternalServerError sends a 500 Internal Server Error response.
func InternalServerError(c *gin.Context, message string) {
	if message == "" {
		message = "Internal server error"
	}
	RespondJSON(c, http.StatusInternalServerError, false, message, nil)
}

// Error sends a custom status error response with optional data payload.
func Error(c *gin.Context, status int, message string, data interface{}) {
	RespondJSON(c, status, false, message, data)
}
