package middleware

import (
	"os"
	"strings"

	"github.com/kamlesh-dev/go-common/response"
	"github.com/gin-gonic/gin"
)

const (
	HeaderAPIKey    = "X-API-Key"
	HeaderProjectID = "X-Project-ID"
	CtxProjectID    = "project_id"
)

// APIKeyAuth validates incoming API Keys and assigns tenant project identity
func APIKeyAuth() gin.HandlerFunc {
	authEnabled := os.Getenv("AUTH_ENABLED") != "false"
	keysMap := parseAPIKeys(os.Getenv("API_KEYS"))

	return func(c *gin.Context) {
		if !authEnabled {
			projectID := c.GetHeader(HeaderProjectID)
			if projectID == "" {
				projectID = "default"
			}
			c.Set(CtxProjectID, projectID)
			c.Next()
			return
		}

		apiKey := c.GetHeader(HeaderAPIKey)
		if apiKey == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				apiKey = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if apiKey == "" {
			response.Unauthorized(c, "API Key is required. Pass 'X-API-Key' or 'Authorization: Bearer <key>' header.")
			c.Abort()
			return
		}

		projectID, exists := keysMap[apiKey]
		if !exists {
			response.Unauthorized(c, "Invalid or unrecognized API Key.")
			c.Abort()
			return
		}

		// Tenant identity is strictly derived from the verified API key to prevent IDOR / spoofing
		c.Set(CtxProjectID, projectID)
		c.Next()
	}
}

// GetProjectID retrieves the project ID from Gin context
func GetProjectID(c *gin.Context) string {
	if val, ok := c.Get(CtxProjectID); ok {
		if pid, ok := val.(string); ok && pid != "" {
			return pid
		}
	}
	return "default"
}

func parseAPIKeys(raw string) map[string]string {
	result := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		// Only allow default dev key in non-production environments
		if os.Getenv("ENV") != "production" {
			result["acc_dev_secret_key"] = "billpro-crm"
		}
		return result
	}

	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		if strings.Contains(pair, ":") {
			parts := strings.SplitN(pair, ":", 2)
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		} else {
			result[pair] = "default"
		}
	}
	return result
}
