package upload

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Save reads uploaded files from the HTTP request, validates their size and type,
// stores them using the configured storage driver, and returns an array of File structs.
// If 1 file was uploaded, an array of 1 element is returned. If multiple files were uploaded,
// all of them are saved and returned.
func Save(c *gin.Context, optList ...Options) ([]*File, error) {
	if c == nil || c.Request == nil {
		return nil, errors.New("upload: invalid gin context or nil request")
	}

	opts := Options{}
	if len(optList) > 0 {
		opts = optList[0]
	}

	// Apply defaults
	if opts.Folder == "" {
		opts.Folder = "uploads"
	}
	if opts.FieldName == "" {
		opts.FieldName = "file"
	}
	if opts.MaxSizeMB <= 0 {
		opts.MaxSizeMB = 50
	}
	if opts.TenantID == "" {
		opts.TenantID = c.GetString("tenant_id")
		if opts.TenantID == "" {
			opts.TenantID = c.GetString("project_id")
		}
	}

	// Ensure multipart form is parsed (max 64MB memory before disk spill)
	if err := c.Request.ParseMultipartForm(64 * 1024 * 1024); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		return nil, fmt.Errorf("upload: failed to parse multipart form: %w", err)
	}

	fileHeaders := extractFileHeaders(c, opts.FieldName)
	if len(fileHeaders) == 0 {
		return nil, fmt.Errorf("upload: no file found in request under field '%s'", opts.FieldName)
	}

	driver, err := GetDriver()
	if err != nil {
		return nil, fmt.Errorf("upload: storage driver unavailable: %w", err)
	}

	ctx := c.Request.Context()
	maxSizeBytes := opts.MaxSizeMB * 1024 * 1024
	var uploadedFiles []*File

	for _, fh := range fileHeaders {
		// 1. Validate file size
		if fh.Size > maxSizeBytes {
			return nil, fmt.Errorf("upload: file '%s' exceeds maximum allowed size of %d MB", fh.Filename, opts.MaxSizeMB)
		}

		// 2. Detect content type
		contentType := fh.Header.Get("Content-Type")
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = detectContentType(fh)
		}

		// 3. Security checks: block executables, scripts, html
		if IsBlockedFileType(fh.Filename, contentType) {
			return nil, fmt.Errorf("upload: file '%s' is an executable or script and is forbidden for security", fh.Filename)
		}

		// 4. Validate allowed types
		if !ValidateAllowed(fh.Filename, contentType, opts.AllowedTypes) {
			return nil, fmt.Errorf("upload: file type of '%s' is not in the allowed list", fh.Filename)
		}

		// 5. Generate collision-resistant unique key
		key := GenerateKey(opts.TenantID, opts.Folder, fh.Filename)

		// 6. Open file stream
		src, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("upload: failed to open uploaded file '%s': %w", fh.Filename, err)
		}

		// 7. Store using driver
		err = driver.Put(ctx, key, src, fh.Size, contentType)
		src.Close()
		if err != nil {
			return nil, fmt.Errorf("upload: failed to save '%s': %w", fh.Filename, err)
		}

		// 8. Build public URL
		fileURL := driver.PublicURL(key)
		if opts.IsPublic != nil && !*opts.IsPublic {
			fileURL = ""
		}

		uploadedFiles = append(uploadedFiles, &File{
			Name:        SafeFileName(fh.Filename),
			URL:         fileURL,
			Key:         key,
			Size:        fh.Size,
			ContentType: contentType,
			UploadedAt:  time.Now().UTC(),
		})
	}

	return uploadedFiles, nil
}

// Delete removes one or more files from storage given an array of file keys.
func Delete(ctx context.Context, fileKeys []string) error {
	if len(fileKeys) == 0 {
		return nil
	}

	driver, err := GetDriver()
	if err != nil {
		return fmt.Errorf("upload: storage driver unavailable: %w", err)
	}

	return driver.Delete(ctx, fileKeys)
}

// Presign generates a signed URL for direct client-to-storage upload (e.g. for large 500MB+ files).
func Presign(c *gin.Context, opts PresignOptions) (*PresignResult, error) {
	if opts.FileName == "" {
		return nil, errors.New("upload: fileName is required for presigning")
	}

	if opts.Folder == "" {
		opts.Folder = "uploads"
	}
	if opts.TenantID == "" && c != nil {
		opts.TenantID = c.GetString("tenant_id")
		if opts.TenantID == "" {
			opts.TenantID = c.GetString("project_id")
		}
	}
	if opts.ExpiresIn <= 0 {
		opts.ExpiresIn = 15 * time.Minute
	}
	if opts.FileType == "" {
		opts.FileType = "application/octet-stream"
	}

	if IsBlockedFileType(opts.FileName, opts.FileType) {
		return nil, errors.New("upload: file extension or MIME type is forbidden for security")
	}

	driver, err := GetDriver()
	if err != nil {
		return nil, fmt.Errorf("upload: storage driver unavailable: %w", err)
	}

	key := GenerateKey(opts.TenantID, opts.Folder, opts.FileName)
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	uploadURL, err := driver.PresignPut(ctx, key, opts.FileType, opts.ExpiresIn)
	if err != nil {
		return nil, err
	}

	publicURL := driver.PublicURL(key)
	if opts.IsPublic != nil && !*opts.IsPublic {
		publicURL = ""
	}

	return &PresignResult{
		UploadURL: uploadURL,
		Key:       key,
		URL:       publicURL,
		ExpiresIn: int64(opts.ExpiresIn.Seconds()),
	}, nil
}

// Helper: extracts file headers from request under fieldName or fallbacks
func extractFileHeaders(c *gin.Context, fieldName string) []*multipart.FileHeader {
	if c.Request.MultipartForm == nil {
		return nil
	}

	// 1. Check exact field name
	if files, ok := c.Request.MultipartForm.File[fieldName]; ok && len(files) > 0 {
		return files
	}

	// 2. Check common alternative names
	alternatives := []string{"files", "upload", "attachment", "attachments", "media"}
	for _, alt := range alternatives {
		if files, ok := c.Request.MultipartForm.File[alt]; ok && len(files) > 0 {
			return files
		}
	}

	// 3. If there is only one key in the form, use its files
	if len(c.Request.MultipartForm.File) == 1 {
		for _, files := range c.Request.MultipartForm.File {
			return files
		}
	}

	return nil
}

// Helper: detects content type from first 512 bytes
func detectContentType(fh *multipart.FileHeader) string {
	f, err := fh.Open()
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return "application/octet-stream"
	}

	return http.DetectContentType(buf[:n])
}
