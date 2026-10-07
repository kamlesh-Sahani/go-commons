package upload

import "time"

// Options configures file upload behavior for Save.
type Options struct {
	Folder       string   // Target directory (e.g. "invoices"). Default: "uploads"
	FieldName    string   // Form field name. Default: "file"
	MaxSizeMB    int64    // Max size in MB. Default: 50MB
	AllowedTypes []string // e.g. []string{"image/png", ".pdf", "jpg"}
	TenantID     string   // Optional project/tenant ID for path scoping
}

// File represents a saved file.
type File struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

// PresignOptions configures direct S3 upload presigning.
type PresignOptions struct {
	FileName    string        `json:"filename"`
	ContentType string        `json:"contentType"`
	Size        int64         `json:"size"`
	Folder      string        `json:"folder"`
	TenantID    string        `json:"tenantId"`
	ExpiresIn   time.Duration `json:"expiresIn"`
}

// PresignResult contains the S3 target URL and form fields for Presigned POST.
type PresignResult struct {
	URL    string            `json:"url"`
	Fields map[string]string `json:"fields"`
	Key    string            `json:"key"`
}

// VerifyResult contains confirmed file metadata from S3.
type VerifyResult struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	Size        int64  `json:"size,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}
