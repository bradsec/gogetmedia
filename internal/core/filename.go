package core

import (
	"regexp"
	"strings"
	"unicode"
)

// SanitizeFilename cleans a filename by keeping only alphanumeric characters and spaces
func SanitizeFilename(filename string) string {
	// Remove file extension temporarily (only if it's a real extension)
	ext := ""
	if lastDot := strings.LastIndex(filename, "."); lastDot != -1 {
		potentialExt := filename[lastDot:]
		// Only treat it as an extension if it's a common file extension
		// and doesn't contain spaces (real extensions don't have spaces)
		if !strings.Contains(potentialExt, " ") && len(potentialExt) <= 6 {
			ext = potentialExt
			filename = filename[:lastDot]
		}
	}

	// Keep only alphanumeric characters and spaces
	// Remove all emojis, symbols, and special characters
	var result strings.Builder
	for _, r := range filename {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			result.WriteRune(r)
		}
	}
	filename = result.String()

	// Replace multiple spaces with single space
	reg := regexp.MustCompile(`\s+`)
	filename = reg.ReplaceAllString(filename, " ")

	// Trim spaces from beginning and end
	filename = strings.TrimSpace(filename)

	// Check for Windows reserved names
	filename = sanitizeWindowsReservedNames(filename)

	// Ensure filename doesn't exceed reasonable length limits
	if len(filename) > 200 { // Leave room for extension
		filename = filename[:200]
		// Make sure we don't end with a space after truncation
		filename = strings.TrimRight(filename, " ")
	}

	// If filename is empty after sanitization, use a default
	if filename == "" {
		filename = "download"
	}

	return filename + ext
}

// sanitizeWindowsReservedNames handles Windows reserved filenames
func sanitizeWindowsReservedNames(filename string) string {
	// Windows reserved names (case-insensitive)
	windowsReservedNames := []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}

	// Check if filename (without extension) matches any reserved name
	for _, reserved := range windowsReservedNames {
		if strings.EqualFold(filename, reserved) {
			return filename + " file"
		}
	}

	return filename
}

// removeEmojis removes emoji characters from a string
func removeEmojis(input string) string {
	var result strings.Builder
	for _, r := range input {
		// Skip emoji ranges
		if isEmoji(r) {
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// isEmoji checks if a rune is an emoji
func isEmoji(r rune) bool {
	// Common emoji ranges
	return (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
		(r >= 0x1F300 && r <= 0x1F5FF) || // Misc Symbols and Pictographs
		(r >= 0x1F680 && r <= 0x1F6FF) || // Transport and Map
		(r >= 0x1F1E0 && r <= 0x1F1FF) || // Regional indicators
		(r >= 0x2600 && r <= 0x26FF) || // Miscellaneous Symbols
		(r >= 0x2700 && r <= 0x27BF) || // Dingbats
		(r >= 0xFE00 && r <= 0xFE0F) || // Variation Selectors
		(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols and Pictographs
		(r >= 0x1F000 && r <= 0x1F02F) || // Mahjong Tiles
		(r >= 0x1F0A0 && r <= 0x1F0FF) || // Playing Cards
		unicode.Is(unicode.So, r) // Other symbols
}
