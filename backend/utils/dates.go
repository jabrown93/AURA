package utils

import (
	"strconv"
	"time"
)

// Formats a date string to time.Time
func ConvertDateStringToTime(dateStr string) (dt time.Time) {

	if dateStr == "" || dateStr == "0" || dateStr == "undefined" {
		return time.Now()
	}

	// 1) Unix epoch (seconds or milliseconds)
	if isAllDigits(dateStr) {
		if len(dateStr) == 13 {
			// Milliseconds
			ms, err := strconv.ParseInt(dateStr, 10, 64)
			if err == nil {
				return time.Unix(0, ms*int64(time.Millisecond))
			}
		} else if len(dateStr) == 10 {
			// Seconds
			sec, err := strconv.ParseInt(dateStr, 10, 64)
			if err == nil {
				return time.Unix(sec, 0)
			}
		} else if len(dateStr) > 14 {
			// Some systems send YYYYMMDDHHMMSS as digits too; handle that here.
			dt, err := time.Parse("20060102150405", dateStr)
			if err == nil {
				return dt
			}
		}
	}

	// 2) Try RFC3339 format (ISO 8601)
	if dt, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return dt
	}
	if dt, err := time.Parse(time.RFC3339Nano, dateStr); err == nil {
		return dt
	}

	// 3) Date-only (YYYY-MM-DD)
	if dt, err := time.Parse("2006-01-02", dateStr); err == nil {
		return dt
	}

	// 4) Compact format (YYYYMMDDHHMMSS)
	if dt, err := time.Parse("20060102150405", dateStr); err == nil {
		return dt
	}

	// 5) Common SQL-ish / Go formats
	// "2006-01-02 15:04:05"
	if dt, err := time.Parse("2006-01-02 15:04:05", dateStr); err == nil {
		return dt.UTC()
	}
	// "2006-01-02 15:04:05 -0700 MST" (time.Time.String())
	if dt, err := time.Parse("2006-01-02 15:04:05 -0700 MST", dateStr); err == nil {
		return dt.UTC()
	}

	// If all parsing attempts fail, return current time
	return time.Now()
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return len(s) > 0
}
