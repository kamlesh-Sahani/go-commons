package utils

import (
	"github.com/gin-gonic/gin"
)

const (
	CtxUserIDKey   = "user_id"
	CtxTenantIDKey = "tenant_id"
	CtxRoleKey     = "user_role"
	CtxEmailKey    = "user_email"
)

// SetAuthContext attaches authenticated identity details to the Gin context.
func SetAuthContext(c *gin.Context, userID, tenantID, role, email string) {
	c.Set(CtxUserIDKey, userID)
	c.Set(CtxTenantIDKey, tenantID)
	c.Set(CtxRoleKey, role)
	c.Set(CtxEmailKey, email)
}

// GetUserID retrieves the authenticated User ID from context.
func GetUserID(c *gin.Context) string {
	if val, ok := c.Get(CtxUserIDKey); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// GetTenantID retrieves the current Tenant ID from context.
func GetTenantID(c *gin.Context) string {
	if val, ok := c.Get(CtxTenantIDKey); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// GetRole retrieves the user role from context.
func GetRole(c *gin.Context) string {
	if val, ok := c.Get(CtxRoleKey); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// GetEmail retrieves the user email from context.
func GetEmail(c *gin.Context) string {
	if val, ok := c.Get(CtxEmailKey); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// GetClientIP extracts the real client IP securely via Gin's proxy-aware ClientIP().
// Configure trusted reverse proxy CIDRs in Gin using router.SetTrustedProxies() in production.
func GetClientIP(c *gin.Context) string {
	return c.ClientIP()
}
