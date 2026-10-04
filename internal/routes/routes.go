package routes

import (
	"github.com/kamlesh-dev/go-common/internal/handlers"
	"github.com/kamlesh-dev/go-common/middleware"
	"github.com/kamlesh-dev/go-common/response"
	"github.com/gin-gonic/gin"
)

// SetupRoutes registers all hosted server endpoints.
func SetupRoutes(router *gin.Engine) {
	// Root service discovery
	router.GET("/", func(c *gin.Context) {
		response.Success(c, "Accurex Common Media & Upload API Gateway", gin.H{
			"version": "1.0.0",
			"status":  "healthy",
		})
	})

	// Health check
	router.GET("/health", func(c *gin.Context) {
		response.Success(c, "healthy", nil)
	})

	// Protected Upload Routes
	uploads := router.Group("/api/v1/upload")
	uploads.Use(middleware.APIKeyAuth())
	{
		uploads.POST("/presigned-url", handlers.GeneratePresignedUpload)
		uploads.POST("/confirm", handlers.ConfirmUpload)
		uploads.POST("/presigned-download-url", handlers.GeneratePresignedDownload)
		uploads.GET("/info", handlers.GetMetadata)
		uploads.DELETE("", handlers.DeleteFile)
		uploads.POST("/batch-delete", handlers.BatchDeleteFiles)
	}
}
