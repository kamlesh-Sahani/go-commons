package utils

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"
	"unicode"
)

var (
	nonAlphaNumericRegex = regexp.MustCompile(`[^a-z0-9]+`)
	multipleDashRegex    = regexp.MustCompile(`-+`)
)

const charsetAlphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// Slugify converts any title or name into a clean, URL-friendly slug.
// Example: "BillPro Invoice #101 — Paid!" -> "billpro-invoice-101-paid"
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Replace non-alphanumeric with hyphens
	s = nonAlphaNumericRegex.ReplaceAllString(s, "-")
	// Collapse multiple consecutive hyphens
	s = multipleDashRegex.ReplaceAllString(s, "-")
	// Trim hyphens from beginning and end
	return strings.Trim(s, "-")
}

// MaskEmail masks an email address for privacy and logs.
// Example: "kamlesh.kumar@gmail.com" -> "k***r@gmail.com"
func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 || len(parts[0]) == 0 {
		return email
	}

	runes := []rune(parts[0])
	domain := parts[1]

	if len(runes) == 1 {
		return string(runes[0]) + "***@" + domain
	}
	if len(runes) == 2 {
		return string(runes[0]) + "***@" + domain
	}

	return string(runes[0]) + "***" + string(runes[len(runes)-1]) + "@" + domain
}

// MaskCard masks a card number, revealing only the last 4 digits.
// Example: "4111222233334444" -> "•••• •••• •••• 4444"
func MaskCard(cardNumber string) string {
	clean := strings.ReplaceAll(cardNumber, " ", "")
	clean = strings.ReplaceAll(clean, "-", "")

	if len(clean) < 4 {
		return "••••"
	}

	last4 := clean[len(clean)-4:]
	return "•••• •••• •••• " + last4
}

// RandomString generates a cryptographically secure random alphanumeric string of specified length.
func RandomString(length int) string {
	if length <= 0 {
		return ""
	}

	b := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charsetAlphanumeric)))

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return ""
		}
		b[i] = charsetAlphanumeric[num.Int64()]
	}

	return string(b)
}

// Truncate cleanly truncates a string with an ellipsis if it exceeds maxLen.
// Uses rune slicing to prevent splitting multi-byte UTF-8 characters.
func Truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// ToSnakeCase converts camelCase or PascalCase to snake_case.
// Uses rune-based indexing to properly support Unicode characters.
func ToSnakeCase(s string) string {
	runes := []rune(s)
	var res []rune
	n := len(runes)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 && (unicode.IsLower(runes[i-1]) || (i+1 < n && unicode.IsLower(runes[i+1]))) {
				res = append(res, '_')
			}
			res = append(res, unicode.ToLower(r))
		} else {
			res = append(res, r)
		}
	}
	return string(res)
}
