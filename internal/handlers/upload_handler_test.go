package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kamlesh-dev/go-common/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
	_ = os.Setenv("AWS_S3_BUCKET", "test-bucket")
	_ = os.Setenv("AWS_REGION", "us-east-1")
}

func setupTestRouter() *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// Mock tenant identity from header for unit tests
		projectID := c.GetHeader("X-Project-ID")
		if projectID == "" {
			projectID = "billpro"
		}
		c.Set(middleware.CtxProjectID, projectID)
		c.Next()
	})

	r.POST("/api/v1/upload/presigned-url", GeneratePresignedUpload)
	r.POST("/api/v1/upload/confirm", ConfirmUpload)
	r.POST("/api/v1/upload/presigned-download-url", GeneratePresignedDownload)
	r.GET("/api/v1/upload/info", GetMetadata)
	r.DELETE("/api/v1/upload", DeleteFile)
	r.POST("/api/v1/upload/batch-delete", BatchDeleteFiles)

	return r
}

func TestGeneratePresignedUpload_SecurityValidation(t *testing.T) {
	router := setupTestRouter()

	t.Run("Rejects dangerous executable/script MIME types", func(t *testing.T) {
		dangerousTypes := []string{"text/html", "application/javascript", "image/svg+xml", "application/x-sh"}
		for _, dt := range dangerousTypes {
			payload, _ := json.Marshal(map[string]interface{}{
				"fileName": "exploit.txt",
				"fileType": dt,
				"fileSize": 1024,
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/upload/presigned-url", bytes.NewBuffer(payload))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 Bad Request for MIME %s, got %d", dt, w.Code)
			}
		}
	})

	t.Run("Rejects missing required fields", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"fileName": "",
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/upload/presigned-url", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for empty fileName, got %d", w.Code)
		}
	})
}

func TestTenantIsolationEnforcement(t *testing.T) {
	router := setupTestRouter()

	t.Run("ConfirmUpload rejects cross-tenant file key", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"fileKey": "projects/victim-tenant/invoices/secret.pdf",
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Project-ID", "attacker-tenant")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for cross-tenant access, got %d", w.Code)
		}
	})

	t.Run("Default tenant cannot access other tenant files", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"fileKey": "projects/victim-tenant/invoices/secret.pdf",
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Project-ID", "default")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for default tenant accessing victim-tenant, got %d", w.Code)
		}
	})

	t.Run("Directory traversal via path separator is blocked", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"fileKey": "projects/attacker-tenant/../../victim-tenant/secret.pdf",
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/upload/confirm", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Project-ID", "attacker-tenant")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for path traversal key, got %d", w.Code)
		}
	})

	t.Run("DeleteFile rejects cross-tenant file deletion", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/upload?key=projects/victim-tenant/invoices/secret.pdf", nil)
		req.Header.Set("X-Project-ID", "attacker-tenant")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for cross-tenant deletion, got %d", w.Code)
		}
	})

	t.Run("BatchDeleteFiles rejects if any file key belongs to another tenant", func(t *testing.T) {
		payload, _ := json.Marshal(map[string]interface{}{
			"fileKeys": []string{
				"projects/attacker-tenant/doc1.pdf",
				"projects/victim-tenant/doc2.pdf",
			},
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/upload/batch-delete", bytes.NewBuffer(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Project-ID", "attacker-tenant")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden when batch includes another tenant's file, got %d", w.Code)
		}
	})
}
