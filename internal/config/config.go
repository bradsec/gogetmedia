package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	DownloadPath             string `json:"download_path"`
	MaxConcurrentDownloads   int    `json:"max_concurrent_downloads"`
	YtDlpPath                string `json:"yt_dlp_path"`
	FfmpegPath               string `json:"ffmpeg_path"`
	Port                     int    `json:"port"`
	DefaultVideoFormat       string `json:"default_video_format"`
	DefaultAudioFormat       string `json:"default_audio_format"`
	VerboseLogging           bool   `json:"verbose_logging"`
	CompletedFileExpiryHours int    `json:"completed_file_expiry_hours"`
	EnableHardwareAccel      bool   `json:"enable_hardware_acceleration"`
	OptimizeForLowPower      bool   `json:"optimize_for_low_power"`
}

func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	downloadPath := filepath.Join(homeDir, "Downloads", "gogetmedia")

	return &Config{
		DownloadPath:             downloadPath,
		MaxConcurrentDownloads:   3,
		YtDlpPath:                getDefaultYtDlpPath(),
		FfmpegPath:               getDefaultFfmpegPath(),
		Port:                     8080,
		DefaultVideoFormat:       "mp4",
		DefaultAudioFormat:       "mp3",
		VerboseLogging:           false,
		CompletedFileExpiryHours: 72, // 72 hours default
		EnableHardwareAccel:      true,
		OptimizeForLowPower:      false,
	}
}

func getDefaultYtDlpPath() string {
	var binaryName string
	if runtime.GOOS == "windows" {
		binaryName = "yt-dlp.exe"
	} else {
		binaryName = "yt-dlp"
	}
	
	// Try to get the executable's directory first
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		ytdlpPath := filepath.Join(execDir, "assets", "yt-dlp", binaryName)
		if _, err := os.Stat(ytdlpPath); err == nil {
			return ytdlpPath
		}
	}
	
	// Try current working directory
	if wd, err := os.Getwd(); err == nil {
		ytdlpPath := filepath.Join(wd, "assets", "yt-dlp", binaryName)
		if _, err := os.Stat(ytdlpPath); err == nil {
			return ytdlpPath
		}
	}
	
	// Try relative path from working directory
	relPath := filepath.Join("assets", "yt-dlp", binaryName)
	if _, err := os.Stat(relPath); err == nil {
		// Convert to absolute path if possible
		if absPath, err := filepath.Abs(relPath); err == nil {
			return absPath
		}
		return relPath
	}
	
	// Last resort - return the relative path (updater will handle download)
	return relPath
}

func getDefaultFfmpegPath() string {
	// Use system PATH by default
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

func Load(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		config := DefaultConfig()
		if err := config.Save(configPath); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

func (c *Config) Save(configPath string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) Validate() error {
	if c.DownloadPath == "" {
		return fmt.Errorf("download_path cannot be empty")
	}

	if c.MaxConcurrentDownloads <= 0 || c.MaxConcurrentDownloads > 10 {
		return fmt.Errorf("max_concurrent_downloads must be between 1 and 10")
	}

	if c.DefaultVideoFormat == "" {
		return fmt.Errorf("default_video_format cannot be empty")
	}

	if c.DefaultAudioFormat == "" {
		return fmt.Errorf("default_audio_format cannot be empty")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if c.CompletedFileExpiryHours < 0 {
		return fmt.Errorf("completed_file_expiry_hours cannot be negative")
	}

	if err := os.MkdirAll(c.DownloadPath, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	return nil
}
