package upload

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func cleanFileName(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	nameOnly := strings.TrimSuffix(base, ext)

	safe := safeNameRe.ReplaceAllString(nameOnly, "_")
	if len(safe) > 50 {
		safe = safe[:50]
	}
	if safe == "" {
		safe = "file"
	}
	return safe + strings.ToLower(ext)
}

func buildKey(tenant, folder, filename string) string {
	cleanFolder := safeNameRe.ReplaceAllString(strings.TrimSpace(folder), "-")
	if cleanFolder == "" {
		cleanFolder = "uploads"
	}
	now := time.Now().UTC()
	uid := uuid.New().String()[:8]

	if tenant = strings.TrimSpace(tenant); tenant != "" {
		cleanTenant := safeNameRe.ReplaceAllString(tenant, "-")
		return fmt.Sprintf("projects/%s/%s/%s/%s/%s_%s", cleanTenant, cleanFolder, now.Format("2006"), now.Format("01"), uid, filename)
	}
	return fmt.Sprintf("%s/%s/%s/%s_%s", cleanFolder, now.Format("2006"), now.Format("01"), uid, filename)
}

func isBlocked(ext string) bool {
	blocked := map[string]bool{
		".exe": true, ".bat": true, ".sh": true, ".php": true, ".py": true,
		".rb": true, ".js": true, ".html": true, ".htm": true, ".svg": true,
		".cmd": true, ".dll": true, ".so": true,
	}
	return blocked[ext]
}

func isAllowed(filename, ctype string, allowed []string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	normMime := strings.ToLower(strings.TrimSpace(ctype))

	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == ext || "."+a == ext || a == normMime {
			return true
		}
		if strings.HasSuffix(a, "/*") && strings.HasPrefix(normMime, strings.TrimSuffix(a, "/*")+"/") {
			return true
		}
	}
	return false
}
