package core

import (
	"os/exec"
	"regexp"
	"strings"
)

type VersionInfo struct {
	YtDlpVersion  string `json:"yt_dlp"`
	FfmpegVersion string `json:"ffmpeg"`
}

func GetVersionInfo(ytdlpPath, ffmpegPath string) *VersionInfo {
	versions := &VersionInfo{}

	// Get yt-dlp version
	if ytdlpVersion := getYtDlpVersion(ytdlpPath); ytdlpVersion != "" {
		versions.YtDlpVersion = ytdlpVersion
	}

	// Get ffmpeg version
	if ffmpegVersion := getFfmpegVersion(ffmpegPath); ffmpegVersion != "" {
		versions.FfmpegVersion = ffmpegVersion
	}

	return versions
}

func getYtDlpVersion(path string) string {
	// Try the configured path first
	if version := tryGetYtDlpVersion(path); version != "" {
		return version
	}

	// Try system PATH
	if version := tryGetYtDlpVersion("yt-dlp"); version != "" {
		return version
	}

	return ""
}

func tryGetYtDlpVersion(path string) string {
	cmd := exec.Command(path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}

func getFfmpegVersion(path string) string {
	// Try the configured path first
	if version := tryGetFfmpegVersion(path); version != "" {
		return version
	}

	// Try system PATH
	if version := tryGetFfmpegVersion("ffmpeg"); version != "" {
		return version
	}

	return ""
}

func tryGetFfmpegVersion(path string) string {
	cmd := exec.Command(path, "-version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Extract version from ffmpeg output
	re := regexp.MustCompile(`ffmpeg version ([^\s]+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func CheckFfmpegAvailable(path string) bool {
	// Try the configured path first
	if isAvailable(path) {
		return true
	}

	// Try system PATH
	return isAvailable("ffmpeg")
}

func isAvailable(command string) bool {
	cmd := exec.Command(command, "-version")
	err := cmd.Run()
	return err == nil
}
