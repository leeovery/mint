package presenter

import "fmt"

// formatSize writes a byte count in decimal units: plain bytes below 1 KB, KB below
// 1 MB, MB from there, with one decimal place below 10 and a whole number from 10
// up. A count that rounds up to the next unit is written in that unit.
func formatSize(bytes int) string {
	switch {
	case bytes < 1_000:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 999_500:
		return scaledSize(float64(bytes)/1e3, "KB")
	default:
		return scaledSize(float64(bytes)/1e6, "MB")
	}
}

func scaledSize(value float64, unit string) string {
	if value < 9.95 {
		return fmt.Sprintf("%.1f %s", value, unit)
	}
	return fmt.Sprintf("%.0f %s", value, unit)
}
