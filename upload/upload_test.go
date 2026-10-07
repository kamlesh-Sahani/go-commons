package upload_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/kamlesh-Sahani/go-commons/upload"
)

func setupTestRouter() (*gin.Engine, string) {
	gin.SetMode(gin.TestMode)
	tmpDir, _ := os.MkdirTemp("", "upload_test_*")
	os.Setenv("UPLOAD_LOCAL_PATH", tmpDir)
	os.Setenv("UPLOAD_STORAGE_DRIVER", "local")
	r := gin.New()
	return r, tmpDir
}

func createMultipartRequest(fieldName string, files map[string][]byte) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for filename, content := range files {
		part, err := writer.CreateFormFile(fieldName, filename)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(content); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestUploadSave_SingleFile(t *testing.T) {
	r, tmpDir := setupTestRouter()
	defer os.RemoveAll(tmpDir)

	var savedFiles []*upload.File
	var saveErr error

	r.POST("/upload", func(c *gin.Context) {
		savedFiles, saveErr = upload.Save(c, upload.Options{
			Folder:    "invoices",
			MaxSizeMB: 5,
		})
		c.Status(http.StatusOK)
	})

	req, _ := createMultipartRequest("file", map[string][]byte{
		"invoice_101.pdf": []byte("%PDF-1.4 test invoice content"),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if saveErr != nil {
		t.Fatalf("unexpected error: %v", saveErr)
	}
	if len(savedFiles) != 1 {
		t.Fatalf("expected 1 file in array, got %d", len(savedFiles))
	}

	f := savedFiles[0]
	if !strings.HasPrefix(f.Name, "invoice_101") {
		t.Errorf("unexpected file name: %s", f.Name)
	}
	if !strings.HasPrefix(f.Key, "invoices/") {
		t.Errorf("key should start with folder 'invoices/', got: %s", f.Key)
	}
	if f.Size != int64(len("%PDF-1.4 test invoice content")) {
		t.Errorf("unexpected size: %d", f.Size)
	}
}

func TestUploadSave_MultipleFiles(t *testing.T) {
	r, tmpDir := setupTestRouter()
	defer os.RemoveAll(tmpDir)

	var savedFiles []*upload.File
	var saveErr error

	r.POST("/upload", func(c *gin.Context) {
		savedFiles, saveErr = upload.Save(c, upload.Options{
			Folder: "photos",
		})
		c.Status(http.StatusOK)
	})

	req, _ := createMultipartRequest("file", map[string][]byte{
		"photo1.png": []byte("\x89PNG\r\n\x1a\nphoto1"),
		"photo2.png": []byte("\x89PNG\r\n\x1a\nphoto2"),
		"photo3.png": []byte("\x89PNG\r\n\x1a\nphoto3"),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if saveErr != nil {
		t.Fatalf("unexpected error: %v", saveErr)
	}
	if len(savedFiles) != 3 {
		t.Fatalf("expected 3 files, got %d", len(savedFiles))
	}
}

func TestUploadSave_TenantScoping(t *testing.T) {
	r, tmpDir := setupTestRouter()
	defer os.RemoveAll(tmpDir)

	var savedFiles []*upload.File
	r.POST("/upload", func(c *gin.Context) {
		c.Set("tenant_id", "billing-corp")
		savedFiles, _ = upload.Save(c, upload.Options{
			Folder: "reports",
		})
		c.Status(http.StatusOK)
	})

	req, _ := createMultipartRequest("file", map[string][]byte{
		"report.pdf": []byte("%PDF-1.4 data"),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(savedFiles) != 1 {
		t.Fatalf("expected 1 file, got %d", len(savedFiles))
	}
	if !strings.HasPrefix(savedFiles[0].Key, "projects/billing-corp/reports/") {
		t.Errorf("expected tenant-scoped key, got: %s", savedFiles[0].Key)
	}
}

func TestUploadSave_RejectsBlockedExtensions(t *testing.T) {
	r, tmpDir := setupTestRouter()
	defer os.RemoveAll(tmpDir)

	var saveErr error
	r.POST("/upload", func(c *gin.Context) {
		_, saveErr = upload.Save(c)
		c.Status(http.StatusOK)
	})

	req, _ := createMultipartRequest("file", map[string][]byte{
		"malicious_script.sh": []byte("#!/bin/bash\necho hack"),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if saveErr == nil {
		t.Fatalf("expected error for .sh file, got nil")
	}
	if !strings.Contains(saveErr.Error(), "not allowed for security") {
		t.Errorf("unexpected error message: %v", saveErr)
	}
}

func TestUploadSave_RejectsOversizedFile(t *testing.T) {
	r, tmpDir := setupTestRouter()
	defer os.RemoveAll(tmpDir)

	var saveErr error
	r.POST("/upload", func(c *gin.Context) {
		_, saveErr = upload.Save(c, upload.Options{
			MaxSizeMB: 1, // 1MB limit
		})
		c.Status(http.StatusOK)
	})

	bigPayload := make([]byte, 2*1024*1024) // 2MB
	req, _ := createMultipartRequest("file", map[string][]byte{
		"large.pdf": bigPayload,
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if saveErr == nil {
		t.Fatalf("expected error for oversized file, got nil")
	}
	if !strings.Contains(saveErr.Error(), "exceeds limit") {
		t.Errorf("unexpected error message: %v", saveErr)
	}
}

func TestUploadDelete_ArrayOfKeys(t *testing.T) {
	_, tmpDir := setupTestRouter()
	defer os.RemoveAll(tmpDir)

	key1 := "uploads/file1.txt"
	key2 := "uploads/file2.txt"

	full1 := filepath.Join(tmpDir, filepath.FromSlash(key1))
	full2 := filepath.Join(tmpDir, filepath.FromSlash(key2))

	_ = os.MkdirAll(filepath.Dir(full1), 0755)
	_ = os.WriteFile(full1, []byte("content1"), 0644)
	_ = os.WriteFile(full2, []byte("content2"), 0644)

	err := upload.Delete(context.Background(), []string{key1, key2})
	if err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	if _, err := os.Stat(full1); !os.IsNotExist(err) {
		t.Errorf("expected file1 to be deleted")
	}
	if _, err := os.Stat(full2); !os.IsNotExist(err) {
		t.Errorf("expected file2 to be deleted")
	}
}
