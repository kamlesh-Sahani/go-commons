package upload

import (
	"time"
)

// Options configures the upload behavior for Save.
type Options struct {
	// Folder is the target storage directory (e.g. "invoices", "avatars"). Default: "uploads".
	Folder string

	// FieldName is the multipart form field name to read files from. Default: "file".
	// If files are not found under FieldName, Save will also search under "files" or all form files.
	FieldName string

	// MaxSizeMB is the maximum allowed size per file in megabytes. Default: 50 MB.
	MaxSizeMB int64

	// AllowedTypes can be MIME types (e.g. "image/png", "application/pdf")
	// or file extensions (e.g. ".png", "jpg", "pdf").
	// If empty, all non-dangerous file types are accepted.
	AllowedTypes []string

	// TenantID is an optional identifier (e.g. "billing", "tenant-1") for multi-tenant path isolation.
	TenantID string

	// IsPublic specifies whether to generate a public CDN/S3 URL. Default: true.
	IsPublic *bool
}

// File represents an uploaded and stored file.
type File struct {
	Name        string    `json:"name"`        // Original sanitized file name
	URL         string    `json:"url"`         // Public URL or storage path
	Key         string    `json:"key"`         // Unique object key in storage
	Size        int64     `json:"size"`        // File size in bytes
	ContentType string    `json:"contentType"` // Detected/provided MIME type
	UploadedAt  time.Time `json:"uploadedAt"`  // Timestamp when uploaded
}

// PresignOptions configures direct presigned upload URL generation.
type PresignOptions struct {
	FileName  string        `json:"fileName"`
	FileType  string        `json:"fileType"`
	FileSize  int64         `json:"fileSize,omitempty"`
	Folder    string        `json:"folder,omitempty"`
	TenantID  string        `json:"tenantId,omitempty"`
	ExpiresIn time.Duration `json:"expiresIn,omitempty"`
	IsPublic  *bool         `json:"isPublic,omitempty"`
}

// PresignResult represents the presigned URL response for direct client uploads.
type PresignResult struct {
	UploadURL string `json:"uploadUrl"`
	Key       string `json:"key"`
	URL       string `json:"url"`
	ExpiresIn int64  `json:"expiresIn"`
}
