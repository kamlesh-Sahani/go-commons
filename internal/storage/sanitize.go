package storage

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var safeFileNameRegex = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// GenerateObjectKey creates an organized, isolated key:
// projects/{projectID}/{folder}/{YYYY}/{MM}/{uuid}_{sanitizedFileName}
func GenerateObjectKey(projectID, folder, originalFileName string) string {
	cleanProjectID := SanitizeString(projectID)
	if cleanProjectID == "" {
		cleanProjectID = "default"
	}

	cleanFolder := SanitizeString(folder)
	if cleanFolder == "" {
		cleanFolder = "uploads"
	}

	now := time.Now().UTC()
	year := now.Format("2006")
	month := now.Format("01")

	cleanFileName := SafeFileName(originalFileName)
	uniquePrefix := uuid.New().String()[:8]

	return fmt.Sprintf("projects/%s/%s/%s/%s/%s_%s", cleanProjectID, cleanFolder, year, month, uniquePrefix, cleanFileName)
}

// SanitizeString cleans a string for safe path segments
func SanitizeString(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	return safeFileNameRegex.ReplaceAllString(s, "-")
}

// SafeFileName extracts basename and removes illegal characters from both name and extension
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
