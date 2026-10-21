// Package bytesize writes byte counts the way mint narrates sizes to people and to the
// AI alike.
package bytesize

import "fmt"

// Format writes a byte count in decimal units: plain bytes below 1 KB, KB below 1 MB,
// MB from there, with one decimal place below 10 and a whole number from 10 up. A count
// that rounds up to the next unit is written in that unit.
func Format(bytes int) string {
	switch {
	case bytes < 1_000:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 999_500:
		return scaled(float64(bytes)/1e3, "KB")
	default:
		return scaled(float64(bytes)/1e6, "MB")
	}
}

func scaled(value float64, unit string) string {
	if value < 9.95 {
		return fmt.Sprintf("%.1f %s", value, unit)
	}
	return fmt.Sprintf("%.0f %s", value, unit)
}
