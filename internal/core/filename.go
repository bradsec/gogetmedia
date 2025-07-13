package core

import (
	"regexp"
	"runtime"
	"strings"
	"unicode"
)

// SanitizeFilename cleans a filename by removing special characters, emojis,
// converting to lowercase, and replacing spaces with underscores
func SanitizeFilename(filename string) string {
	// Remove file extension temporarily
	ext := ""
	if lastDot := strings.LastIndex(filename, "."); lastDot != -1 {
		ext = filename[lastDot:]
		filename = filename[:lastDot]
	}

	// Remove emojis and other unicode symbols
	filename = removeEmojis(filename)

	// Remove special characters (Windows-compatible)
	var reg *regexp.Regexp
	if runtime.GOOS == "windows" {
		// Windows forbidden characters: < > : " | ? * \ / and control characters
		reg = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
		filename = reg.ReplaceAllString(filename, "")
	} else {
		// Unix-like systems: remove most special characters except spaces, hyphens, underscores
		reg = regexp.MustCompile(`[^\w\s\-]`)
		filename = reg.ReplaceAllString(filename, "")
	}

	// Replace multiple spaces with single space
	reg = regexp.MustCompile(`\s+`)
	filename = reg.ReplaceAllString(filename, " ")

	// Trim spaces
	filename = strings.TrimSpace(filename)

	// Convert to lowercase
	filename = strings.ToLower(filename)

	// Replace spaces with underscores
	filename = strings.ReplaceAll(filename, " ", "_")

	// Replace multiple underscores with single underscore
	reg = regexp.MustCompile(`_{2,}`)
	filename = reg.ReplaceAllString(filename, "_")

	// Remove leading/trailing underscores
	filename = strings.Trim(filename, "_")

	// Windows-specific sanitization (apply on all platforms for consistency)
	// Remove trailing dots and spaces (Windows doesn't allow these)
	filename = strings.TrimRight(filename, ". ")

	// Check for Windows reserved names
	filename = sanitizeWindowsReservedNames(filename)

	// Ensure filename doesn't exceed reasonable length limits
	if len(filename) > 200 { // Leave room for extension
		filename = filename[:200]
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
			return filename + "_file"
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
