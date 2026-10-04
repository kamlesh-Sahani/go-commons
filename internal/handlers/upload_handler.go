package handlers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kamlesh-dev/go-common/internal/storage"
	"github.com/kamlesh-dev/go-common/middleware"
	"github.com/kamlesh-dev/go-common/response"
	"github.com/gin-gonic/gin"
)

var blockedMIMETypes = map[string]bool{
	"text/html":                 true,
	"application/xhtml+xml":     true,
	"application/javascript":    true,
	"text/javascript":           true,
	"image/svg+xml":             true,
	"application/x-msdownload":  true,
	"application/x-sh":          true,
	"application/x-executable":  true,
}

func validateFileKey(projectID, fileKey string) bool {
	cleanKey := filepath.ToSlash(filepath.Clean(fileKey))
	expectedPrefix := fmt.Sprintf("projects/%s/", storage.SanitizeString(projectID))
	return strings.HasPrefix(cleanKey, expectedPrefix) && !strings.Contains(cleanKey, "..")
}

type presignRequest struct {
	FileName         string `json:"fileName" binding:"required"`
	FileType         string `json:"fileType" binding:"required"`
	FileSize         int64  `json:"fileSize"`
	Folder           string `json:"folder"`
	IsPublic         *bool  `json:"isPublic"`
	ExpiresInMinutes int    `json:"expiresInMinutes"`
}

type confirmRequest struct {
	FileKey string `json:"fileKey" binding:"required"`
}

type presignDownloadRequest struct {
	FileKey          string `json:"fileKey" binding:"required"`
	ExpiresInMinutes int    `json:"expiresInMinutes"`
}

type batchDeleteRequest struct {
	FileKeys []string `json:"fileKeys" binding:"required,min=1"`
}

// GeneratePresignedUpload handles POST /api/v1/upload/presigned-url
func GeneratePresignedUpload(c *gin.Context) {
	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request payload: "+err.Error())
		return
	}

	normMime := strings.ToLower(strings.TrimSpace(req.FileType))
	if blockedMIMETypes[normMime] {
		response.BadRequest(c, "Upload of executable or script file types is forbidden for security")
		return
	}

	maxMB := getMaxFileSizeMB()
	if req.FileSize > maxMB*1024*1024 {
		response.BadRequest(c, fmt.Sprintf("File size exceeds maximum allowed limit of %d MB", maxMB))
		return
	}

	projectID := middleware.GetProjectID(c)
	fileKey := storage.GenerateObjectKey(projectID, req.Folder, req.FileName)

	expiryMinutes := req.ExpiresInMinutes
	if expiryMinutes <= 0 {
		expiryMinutes = 15
	}
	expireDuration := time.Duration(expiryMinutes) * time.Minute

	uploadURL, err := storage.GeneratePresignedUpload(c.Request.Context(), fileKey, req.FileType, expireDuration)
	if err != nil {
		log.Printf("[ERROR] GeneratePresignedUpload failed: %v", err)
		response.InternalServerError(c, "Failed to generate presigned upload URL")
		return
	}

	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}

	var permanentURL string
	if isPublic {
		permanentURL = storage.BuildPublicURL(fileKey)
	}

	response.Success(c, "Presigned upload URL generated successfully", gin.H{
		"uploadUrl": uploadURL,
		"fileKey":   fileKey,
		"fileUrl":   permanentURL,
		"expiresIn": int64(expireDuration.Seconds()),
		"headers": gin.H{
			"Content-Type": req.FileType,
		},
	})
}

// ConfirmUpload handles POST /api/v1/upload/confirm
func ConfirmUpload(c *gin.Context) {
	var req confirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Missing or invalid 'fileKey' parameter")
		return
	}

	projectID := middleware.GetProjectID(c)
	if !validateFileKey(projectID, req.FileKey) {
		response.Forbidden(c, fmt.Sprintf("Access denied: file does not belong to project %s", projectID))
		return
	}

	metadata, err := storage.ConfirmObject(c.Request.Context(), req.FileKey)
	if err != nil {
		log.Printf("[ERROR] ConfirmObject failed: %v", err)
		response.InternalServerError(c, "Failed to confirm upload")
		return
	}

	response.Success(c, "File upload confirmed and marked permanent", gin.H{
		"fileKey":     metadata.Key,
		"fileUrl":     storage.BuildPublicURL(metadata.Key),
		"size":        metadata.Size,
		"contentType": metadata.ContentType,
		"status":      metadata.Status,
	})
}

// GeneratePresignedDownload handles POST /api/v1/upload/presigned-download-url
func GeneratePresignedDownload(c *gin.Context) {
	var req presignDownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Missing or invalid 'fileKey' parameter")
		return
	}

	projectID := middleware.GetProjectID(c)
	if !validateFileKey(projectID, req.FileKey) {
		response.Forbidden(c, fmt.Sprintf("Access denied: file does not belong to project %s", projectID))
		return
	}

	expiryMinutes := req.ExpiresInMinutes
	if expiryMinutes <= 0 {
		expiryMinutes = 15
	}
	expireDuration := time.Duration(expiryMinutes) * time.Minute

	downloadURL, err := storage.GeneratePresignedDownload(c.Request.Context(), req.FileKey, expireDuration)
	if err != nil {
		log.Printf("[ERROR] GeneratePresignedDownload failed: %v", err)
		response.InternalServerError(c, "Failed to generate presigned download URL")
		return
	}

	response.Success(c, "Presigned download URL generated successfully", gin.H{
		"downloadUrl": downloadURL,
		"fileKey":     req.FileKey,
		"expiresIn":   int64(expireDuration.Seconds()),
	})
}

// GetMetadata handles GET /api/v1/upload/info?key=...
func GetMetadata(c *gin.Context) {
	fileKey := c.Query("key")
	if fileKey == "" {
		response.BadRequest(c, "Query parameter 'key' is required")
		return
	}

	projectID := middleware.GetProjectID(c)
	if !validateFileKey(projectID, fileKey) {
		response.Forbidden(c, fmt.Sprintf("Access denied: file does not belong to project %s", projectID))
		return
	}

	metadata, err := storage.GetObjectMetadata(c.Request.Context(), fileKey)
	if err != nil {
		log.Printf("[ERROR] GetObjectMetadata failed: %v", err)
		response.NotFound(c, "File not found or inaccessible")
		return
	}

	response.Success(c, "File metadata retrieved successfully", metadata)
}

// DeleteFile handles DELETE /api/v1/upload?key=...
func DeleteFile(c *gin.Context) {
	fileKey := c.Query("key")
	if fileKey == "" {
		response.BadRequest(c, "Query parameter 'key' is required")
		return
	}

	projectID := middleware.GetProjectID(c)
	if !validateFileKey(projectID, fileKey) {
		response.Forbidden(c, fmt.Sprintf("Access denied: file does not belong to project %s", projectID))
		return
	}

	if err := storage.DeleteS3Object(c.Request.Context(), fileKey); err != nil {
		log.Printf("[ERROR] DeleteS3Object failed: %v", err)
		response.InternalServerError(c, "Failed to delete file from storage")
		return
	}

	response.Success(c, "File deleted successfully from storage", gin.H{"fileKey": fileKey})
}

// BatchDeleteFiles handles POST /api/v1/upload/batch-delete
func BatchDeleteFiles(c *gin.Context) {
	var req batchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request payload: "+err.Error())
		return
	}

	projectID := middleware.GetProjectID(c)
	for _, key := range req.FileKeys {
		if !validateFileKey(projectID, key) {
			response.Forbidden(c, fmt.Sprintf("Access denied: file %s does not belong to project %s", key, projectID))
			return
		}
	}

	if err := storage.DeleteS3Objects(c.Request.Context(), req.FileKeys); err != nil {
		log.Printf("[ERROR] DeleteS3Objects failed: %v", err)
		response.InternalServerError(c, "Failed to batch delete files")
		return
	}

	response.Success(c, "Files deleted successfully", gin.H{"deletedCount": len(req.FileKeys)})
}

func getMaxFileSizeMB() int64 {
	valStr := os.Getenv("MAX_FILE_SIZE_MB")
	if valStr != "" {
		if val, err := strconv.ParseInt(valStr, 10, 64); err == nil && val > 0 {
			return val
		}
	}
	return 500
}
