package upload

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var safeFileNameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

var blockedExtensions = map[string]bool{
	".exe":   true,
	".bat":   true,
	".cmd":   true,
	".sh":    true,
	".bash":  true,
	".php":   true,
	".phtml": true,
	".py":    true,
	".rb":    true,
	".pl":    true,
	".cgi":   true,
	".jar":   true,
	".vbs":   true,
	".js":    true,
	".mjs":   true,
	".html":  true,
	".htm":   true,
	".xhtml": true,
	".svg":   true,
	".msi":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".com":   true,
	".scr":   true,
}

var blockedMIMETypes = map[string]bool{
	"text/html":                true,
	"application/xhtml+xml":    true,
	"application/javascript":   true,
	"text/javascript":          true,
	"image/svg+xml":            true,
	"application/x-msdownload": true,
	"application/x-sh":         true,
	"application/x-executable": true,
	"application/x-bat":        true,
	"application/x-php":        true,
	"text/x-php":               true,
}

// GenerateKey builds a clean, collision-free object key.
// If tenantID is present: projects/{tenantID}/{folder}/{YYYY}/{MM}/{uuid}_{cleanFileName}
// If tenantID is empty:   {folder}/{YYYY}/{MM}/{uuid}_{cleanFileName}
func GenerateKey(tenantID, folder, originalFileName string) string {
	cleanFolder := SanitizeString(folder)
	if cleanFolder == "" {
		cleanFolder = "uploads"
	}

	now := time.Now().UTC()
	year := now.Format("2006")
	month := now.Format("01")

	cleanFileName := SafeFileName(originalFileName)
	uniqueID := uuid.New().String()

	cleanTenant := SanitizeString(tenantID)
	if cleanTenant != "" {
		return fmt.Sprintf("projects/%s/%s/%s/%s/%s_%s", cleanTenant, cleanFolder, year, month, uniqueID, cleanFileName)
	}

	return fmt.Sprintf("%s/%s/%s/%s_%s", cleanFolder, year, month, uniqueID, cleanFileName)
}

// SanitizeString cleans a string for safe path segments.
func SanitizeString(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	return safeFileNameRegex.ReplaceAllString(s, "-")
}

// SafeFileName extracts basename and removes illegal characters.
func SafeFileName(filename string) string {
	filename = filepath.Base(filename)
	ext := filepath.Ext(filename)
	nameOnly := strings.TrimSuffix(filename, ext)

	safeName := safeFileNameRegex.ReplaceAllString(nameOnly, "_")
	runes := []rune(safeName)
	if len(runes) > 60 {
		safeName = string(runes[:60])
	}
	if safeName == "" {
		safeName = "file"
	}

	cleanExt := safeFileNameRegex.ReplaceAllString(ext, "")
	cleanExt = strings.ToLower(cleanExt)
	if cleanExt != "" && !strings.HasPrefix(cleanExt, ".") {
		cleanExt = "." + cleanExt
	}
	return safeName + cleanExt
}

// IsBlockedFileType checks if the file has a forbidden MIME type or dangerous script/executable extension.
func IsBlockedFileType(fileName, mimeType string) bool {
	normMime := strings.ToLower(strings.TrimSpace(mimeType))
	if blockedMIMETypes[normMime] {
		return true
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if blockedExtensions[ext] {
		return true
	}

	return false
}

// ValidateAllowed verifies if the file matches any allowed MIME type or extension.
// If allowed list is empty, returns true (allows everything that is not blocked).
func ValidateAllowed(fileName, mimeType string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	normMime := strings.ToLower(strings.TrimSpace(mimeType))

	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}

		// Check extension (e.g. "png", ".png", "pdf", ".pdf")
		if strings.HasPrefix(a, ".") && ext == a {
			return true
		}
		if !strings.Contains(a, "/") && ext == "."+a {
			return true
		}

		// Check MIME type (e.g. "image/png", "application/pdf", "image/*")
		if strings.HasSuffix(a, "/*") {
			prefix := strings.TrimSuffix(a, "/*")
			if strings.HasPrefix(normMime, prefix+"/") {
				return true
			}
		} else if normMime == a {
			return true
		}
	}

	return false
}
