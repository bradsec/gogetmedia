package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	GitHubAPIURL   = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"
	UpdateCheckURL = "https://github.com/yt-dlp/yt-dlp/releases/latest"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type UpdateInfo struct {
	CurrentVersion  string    `json:"current_version"`
	LatestVersion   string    `json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	LastChecked     time.Time `json:"last_checked"`
}

type YtDlpUpdater struct {
	binPath      string
	assetsDir    string
	versionCache *UpdateInfo
}

func NewYtDlpUpdater(binPath, assetsDir string) *YtDlpUpdater {
	return &YtDlpUpdater{
		binPath:   binPath,
		assetsDir: assetsDir,
	}
}

func (u *YtDlpUpdater) GetCurrentVersion() (string, error) {
	if _, err := os.Stat(u.binPath); os.IsNotExist(err) {
		return "", fmt.Errorf("yt-dlp binary not found at %s", u.binPath)
	}

	// Try to get version from yt-dlp --version
	cmd := exec.Command(u.binPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown", nil // Binary exists but version check failed
	}

	version := strings.TrimSpace(string(output))
	return version, nil
}

func (u *YtDlpUpdater) GetLatestVersion() (string, error) {
	resp, err := http.Get(GitHubAPIURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	return release.TagName, nil
}

func (u *YtDlpUpdater) CheckForUpdates() (*UpdateInfo, error) {
	currentVersion, err := u.GetCurrentVersion()
	if err != nil {
		currentVersion = "unknown"
	}

	latestVersion, err := u.GetLatestVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	updateInfo := &UpdateInfo{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestVersion,
		UpdateAvailable: currentVersion != latestVersion && currentVersion != "unknown",
		LastChecked:     time.Now(),
	}

	u.versionCache = updateInfo
	return updateInfo, nil
}

func (u *YtDlpUpdater) Update() error {
	// Get latest release info
	resp, err := http.Get(GitHubAPIURL)
	if err != nil {
		return fmt.Errorf("failed to fetch latest release info: %w", err)
	}
	defer resp.Body.Close()

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	// Find the appropriate binary for current OS
	downloadURL, err := u.getBinaryURL(release.Assets)
	if err != nil {
		return fmt.Errorf("failed to find binary for current OS: %w", err)
	}

	// Download the binary
	tempFile, err := u.downloadBinary(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer os.Remove(tempFile)

	// Install the binary
	if err := u.installBinary(tempFile); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	return nil
}

func (u *YtDlpUpdater) getBinaryURL(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}) (string, error) {
	var targetName string

	switch runtime.GOOS {
	case "windows":
		targetName = "yt-dlp.exe"
	case "darwin":
		targetName = "yt-dlp_macos"
	case "linux":
		targetName = "yt-dlp"
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	for _, asset := range assets {
		if asset.Name == targetName {
			return asset.BrowserDownloadURL, nil
		}
	}

	return "", fmt.Errorf("no binary found for OS: %s", runtime.GOOS)
}

func (u *YtDlpUpdater) downloadBinary(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create temporary file
	tempFile, err := os.CreateTemp("", "yt-dlp-update-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Copy downloaded content to temp file
	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write downloaded content: %w", err)
	}

	return tempFile.Name(), nil
}

func (u *YtDlpUpdater) installBinary(tempFile string) error {
	// Ensure the assets directory exists
	if err := os.MkdirAll(u.assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}

	// Backup existing binary if it exists
	if _, err := os.Stat(u.binPath); err == nil {
		backupPath := u.binPath + ".backup"
		if err := os.Rename(u.binPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing binary: %w", err)
		}
	}

	// Copy temp file to final location
	if err := u.copyFile(tempFile, u.binPath); err != nil {
		return fmt.Errorf("failed to copy binary to final location: %w", err)
	}

	// Make binary executable (Unix-like systems)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(u.binPath, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	return nil
}

func (u *YtDlpUpdater) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func (u *YtDlpUpdater) GetCachedUpdateInfo() *UpdateInfo {
	return u.versionCache
}
