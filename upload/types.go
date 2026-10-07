package upload

// Options configures file upload behavior.
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
