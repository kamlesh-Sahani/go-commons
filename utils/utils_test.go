package utils

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestValidators(t *testing.T) {
	t.Run("Email validation", func(t *testing.T) {
		if !IsValidEmail("user@example.com") {
			t.Errorf("expected valid email")
		}
		if IsValidEmail("invalid-email") {
			t.Errorf("expected invalid email")
		}
		if !IsDisposableEmail("spam@mailinator.com") {
			t.Errorf("expected disposable email to be identified")
		}
	})

	t.Run("Date validation", func(t *testing.T) {
		if !IsValidDate("2005-04-07") {
			t.Errorf("expected simple calendar date YYYY-MM-DD to be valid")
		}
		if !IsValidDate("2005-04-07T10:00:00.000Z") {
			t.Errorf("expected ISO-8601/RFC3339 date to be valid")
		}
		if IsValidDate("invalid-date") {
			t.Errorf("expected invalid date to fail")
		}
		if IsValidDate("") {
			t.Errorf("expected empty date to fail")
		}
	})

	t.Run("NotBlank and FormatValidationError", func(t *testing.T) {
		type TestForm struct {
			Username string `validate:"required,notblank"`
			Email    string `validate:"required,email"`
			Age      int    `validate:"gte=18"`
		}

		// Test whitespace string fails notblank
		form := TestForm{
			Username: "   ",
			Email:    "invalid-email",
			Age:      15,
		}

		err := Validate.Struct(form)
		if err == nil {
			t.Fatalf("expected validation errors, got nil")
		}

		errMsg := FormatValidationError(err)
		if errMsg == "" {
			t.Errorf("expected formatted error message, got empty string")
		}
	})

	t.Run("Phone validation", func(t *testing.T) {
		if !IsValidPhone("+14155552671") {
			t.Errorf("expected valid international phone")
		}
		if !IsValidPhone("+919876543210") {
			t.Errorf("expected valid indian phone")
		}
		if IsValidPhone("12345") {
			t.Errorf("expected invalid phone to fail")
		}
	})

	t.Run("UUID validation", func(t *testing.T) {
		if !IsValidUUID("550e8400-e29b-41d4-a716-446655440000") {
			t.Errorf("expected valid UUID")
		}
		if IsValidUUID("not-a-uuid") {
			t.Errorf("expected invalid UUID to fail")
		}
	})

	t.Run("URL validation", func(t *testing.T) {
		if !IsValidURL("https://accurex.com/api") {
			t.Errorf("expected valid URL")
		}
		if IsValidURL("ftp://bad") {
			t.Errorf("expected ftp URL to fail")
		}
	})
}

func TestStringUtils(t *testing.T) {
	t.Run("Slugify", func(t *testing.T) {
		slug := Slugify("BillPro Invoice #101 — Paid!")
		if slug != "billpro-invoice-101-paid" {
			t.Errorf("unexpected slug: %s", slug)
		}
	})

	t.Run("MaskEmail", func(t *testing.T) {
		masked := MaskEmail("kamlesh.kumar@gmail.com")
		if masked != "k***r@gmail.com" {
			t.Errorf("unexpected masked email: %s", masked)
		}

		// Edge case: single and double character local part
		if m := MaskEmail("a@gmail.com"); m != "a***@gmail.com" {
			t.Errorf("unexpected single-char masked email: %s", m)
		}
		if m := MaskEmail("ab@gmail.com"); m != "a***@gmail.com" {
			t.Errorf("unexpected two-char masked email: %s", m)
		}
		// Edge case: empty local part should not panic
		if m := MaskEmail("@gmail.com"); m != "@gmail.com" {
			t.Errorf("unexpected empty local-part email: %s", m)
		}
		if m := MaskEmail("invalid-email"); m != "invalid-email" {
			t.Errorf("unexpected invalid email: %s", m)
		}
	})

	t.Run("MaskCard", func(t *testing.T) {
		masked := MaskCard("4111222233334444")
		if masked != "•••• •••• •••• 4444" {
			t.Errorf("unexpected masked card: %s", masked)
		}
	})

	t.Run("RandomString", func(t *testing.T) {
		s := RandomString(16)
		if len(s) != 16 {
			t.Errorf("expected length 16, got %d", len(s))
		}
	})

	t.Run("Truncate UTF-8", func(t *testing.T) {
		// Multi-byte Hindi characters should not be sliced mid-byte
		hindi := "अंतरिक्ष" // 8 runes, 24 bytes
		tr := Truncate(hindi, 5)
		if tr != "अं..." {
			t.Errorf("unexpected truncated UTF-8 string: %s", tr)
		}
		short := Truncate("Hello", 10)
		if short != "Hello" {
			t.Errorf("expected original string, got %s", short)
		}
	})

	t.Run("ToSnakeCase", func(t *testing.T) {
		res := ToSnakeCase("UserProfileView")
		if res != "user_profile_view" {
			t.Errorf("unexpected snake_case: %s", res)
		}
	})
}

func TestTimeUtils(t *testing.T) {
	t.Run("StartOfDay and EndOfDay", func(t *testing.T) {
		now := time.Now()
		start := StartOfDay(now)
		end := EndOfDay(now)

		if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
			t.Errorf("unexpected start of day: %v", start)
		}
		if end.Hour() != 23 || end.Minute() != 59 || end.Second() != 59 {
			t.Errorf("unexpected end of day: %v", end)
		}
	})

	t.Run("ParseDate", func(t *testing.T) {
		parsed, err := ParseDate("2026-10-03")
		if err != nil {
			t.Fatalf("failed to parse date: %v", err)
		}
		if parsed.Year() != 2026 || parsed.Month() != 10 || parsed.Day() != 3 {
			t.Errorf("unexpected date parsed: %v", parsed)
		}
	})

	t.Run("TimeAgo", func(t *testing.T) {
		past := time.Now().Add(-5 * time.Minute)
		if TimeAgo(past) != "5 minutes ago" {
			t.Errorf("unexpected time ago: %s", TimeAgo(past))
		}
	})
}

func TestCurrencyUtils(t *testing.T) {
	t.Run("Cents and decimal conversion", func(t *testing.T) {
		cents := DecimalToCents(19.99)
		if cents != 1999 {
			t.Errorf("expected 1999 cents, got %d", cents)
		}

		dec := CentsToDecimal(1999)
		if dec != 19.99 {
			t.Errorf("expected 19.99, got %f", dec)
		}
	})

	t.Run("FormatCurrency", func(t *testing.T) {
		usd := FormatCurrency(150050, "USD")
		if usd != "$1,500.50" {
			t.Errorf("unexpected formatted USD: %s", usd)
		}

		inr := FormatCurrency(1000000, "INR")
		if inr != "₹10,000.00" {
			t.Errorf("unexpected formatted INR: %s", inr)
		}

		jpy := FormatCurrency(1500, "JPY")
		if jpy != "¥1,500" {
			t.Errorf("unexpected formatted JPY: %s (expected ¥1,500)", jpy)
		}
	})

	t.Run("Tax and Discount calculation", func(t *testing.T) {
		subtotal := int64(10000) // $100.00
		tax := CalculateTax(subtotal, 18.0) // 18%
		if tax != 1800 {
			t.Errorf("expected 1800 tax, got %d", tax)
		}

		discount := CalculateDiscount(subtotal, 10.0) // 10%
		if discount != 1000 {
			t.Errorf("expected 1000 discount, got %d", discount)
		}
	})
}

func TestJWTUtils(t *testing.T) {
	secretKey := "super-secure-jwt-secret-key-12345"
	claims := JWTClaims{
		UserID:   "usr_101",
		Email:    "test@example.com",
		Role:     "admin",
		TenantID: "tenant_abc",
	}

	token, err := GenerateJWT(claims, secretKey, 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	parsedClaims, err := ValidateJWT(token, secretKey)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if parsedClaims.UserID != "usr_101" || parsedClaims.Email != "test@example.com" {
		t.Errorf("claims do not match: %+v", parsedClaims)
	}

	_, err = ValidateJWT(token, "wrong-key")
	if err == nil {
		t.Errorf("expected validation to fail with wrong secret")
	}
}

func TestContextUtils(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	SetAuthContext(c, "u_1", "t_1", "owner", "owner@corp.com")

	if GetUserID(c) != "u_1" {
		t.Errorf("unexpected user id: %s", GetUserID(c))
	}
	if GetTenantID(c) != "t_1" {
		t.Errorf("unexpected tenant id: %s", GetTenantID(c))
	}
	if GetRole(c) != "owner" {
		t.Errorf("unexpected role: %s", GetRole(c))
	}
	if GetEmail(c) != "owner@corp.com" {
		t.Errorf("unexpected email: %s", GetEmail(c))
	}
}
