package utils

import (
	"fmt"
	"math"
	"strings"
)

var (
	currencySymbols = map[string]string{
		"USD": "$",
		"EUR": "€",
		"GBP": "£",
		"INR": "₹",
		"CAD": "CA$",
		"AUD": "AU$",
		"JPY": "¥",
		"KRW": "₩",
	}

	zeroDecimalCurrencies = map[string]bool{
		"BIF": true, "CLP": true, "DJF": true, "GNF": true,
		"JPY": true, "KMF": true, "KRW": true, "MGA": true,
		"PYG": true, "RWF": true, "UGX": true, "VND": true,
		"VUV": true, "XAF": true, "XOF": true, "XPF": true,
	}
)

// IsZeroDecimalCurrency returns true if the ISO currency code has no subunits/cents.
func IsZeroDecimalCurrency(currencyCode string) bool {
	return zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(currencyCode))]
}

// DecimalToCents safely converts a decimal amount (e.g. 19.99) to integer cents (1999).
func DecimalToCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

// CentsToDecimal converts integer cents (1999) to decimal amount (19.99).
func CentsToDecimal(cents int64) float64 {
	return float64(cents) / 100.0
}

// FormatCurrency formats integer cents into a standard currency string (e.g. 150050, "USD" -> "$1,500.50").
// For zero-decimal currencies like JPY, it formats whole units without decimal places (e.g. 1500, "JPY" -> "¥1,500").
func FormatCurrency(amountInCents int64, currencyCode string) string {
	code := strings.ToUpper(strings.TrimSpace(currencyCode))
	symbol, exists := currencySymbols[code]
	if !exists {
		symbol = code + " "
	}

	negative := amountInCents < 0
	absCents := amountInCents
	if negative {
		absCents = -absCents
	}

	if IsZeroDecimalCurrency(code) {
		unitStr := formatNumberWithCommas(absCents)
		formatted := fmt.Sprintf("%s%s", symbol, unitStr)
		if negative {
			return "-" + formatted
		}
		return formatted
	}

	units := absCents / 100
	decimals := absCents % 100

	// Format thousands comma
	unitStr := formatNumberWithCommas(units)

	formatted := fmt.Sprintf("%s%s.%02d", symbol, unitStr, decimals)
	if negative {
		return "-" + formatted
	}
	return formatted
}

// CalculateTax calculates tax amount in integer cents.
func CalculateTax(subtotalInCents int64, taxPercent float64) int64 {
	if taxPercent <= 0 || subtotalInCents <= 0 {
		return 0
	}
	tax := float64(subtotalInCents) * (taxPercent / 100.0)
	return int64(math.Round(tax))
}

// CalculateDiscount calculates discount in integer cents.
func CalculateDiscount(subtotalInCents int64, discountPercent float64) int64 {
	if discountPercent <= 0 || subtotalInCents <= 0 {
		return 0
	}
	discount := float64(subtotalInCents) * (discountPercent / 100.0)
	return int64(math.Round(discount))
}

func formatNumberWithCommas(n int64) string {
	in := fmt.Sprintf("%d", n)
	var out []byte
	l := len(in)
	for i, c := range in {
		out = append(out, byte(c))
		if (l-i-1)%3 == 0 && i != l-1 {
			out = append(out, ',')
		}
	}
	return string(out)
}
