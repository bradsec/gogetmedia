package utils

import (
	"log"
	"os"
)

var (
	VerboseLogging = false
	logger         = log.New(os.Stdout, "", log.LstdFlags)
)

// SetVerboseLogging sets the global verbose logging flag
func SetVerboseLogging(verbose bool) {
	VerboseLogging = verbose
}

// LogInfo logs informational messages only if verbose logging is enabled
func LogInfo(format string, args ...interface{}) {
	if VerboseLogging {
		logger.Printf("[INFO] "+format, args...)
	}
}

// LogError logs error messages (always shown)
func LogError(format string, args ...interface{}) {
	logger.Printf("[ERROR] "+format, args...)
}

// LogWarning logs warning messages (always shown)
func LogWarning(format string, args ...interface{}) {
	logger.Printf("[WARNING] "+format, args...)
}

// LogSuccess logs success messages (always shown)
func LogSuccess(format string, args ...interface{}) {
	logger.Printf("[SUCCESS] "+format, args...)
}
