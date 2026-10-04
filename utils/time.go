package utils

import (
	"fmt"
	"time"
)

// StartOfDay returns 00:00:00.000 on the same date in the given timezone.
func StartOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

// EndOfDay returns 23:59:59.999999999 on the same date in the given timezone.
func EndOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 23, 59, 59, 999999999, t.Location())
}

// StartOfMonth returns the first day of the month at 00:00:00.
func StartOfMonth(t time.Time) time.Time {
	year, month, _ := t.Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
}

// EndOfMonth returns the last nanosecond of the current month.
func EndOfMonth(t time.Time) time.Time {
	firstOfNextMonth := StartOfMonth(t).AddDate(0, 1, 0)
	return firstOfNextMonth.Add(-time.Nanosecond)
}

// ParseDate tries to parse a date string using common formats.
func ParseDate(dateStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"02-01-2006",
		"02/01/2006",
		"01/02/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date string '%s'", dateStr)
}

// TimeAgo returns a human-readable relative duration string (e.g. "2 hours ago").
func TimeAgo(t time.Time) string {
	now := time.Now().UTC()
	diff := now.Sub(t.UTC())

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 48*time.Hour {
		return "yesterday"
	} else {
		days := int(diff.Hours() / 24)
		if days < 30 {
			return fmt.Sprintf("%d days ago", days)
		} else if days < 365 {
			months := days / 30
			if months == 1 {
				return "1 month ago"
			}
			return fmt.Sprintf("%d months ago", months)
		} else {
			years := days / 365
			if years == 1 {
				return "1 year ago"
			}
			return fmt.Sprintf("%d years ago", years)
		}
	}
}
