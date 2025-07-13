package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxConcurrentDownloads != 3 {
		t.Errorf("Expected MaxConcurrentDownloads to be 3, got %d", cfg.MaxConcurrentDownloads)
	}

	if cfg.Port != 8080 {
		t.Errorf("Expected Port to be 8080, got %d", cfg.Port)
	}

	if cfg.DefaultVideoFormat != "mp4" {
		t.Errorf("Expected DefaultVideoFormat to be 'mp4', got '%s'", cfg.DefaultVideoFormat)
	}

	if cfg.DefaultAudioFormat != "mp3" {
		t.Errorf("Expected DefaultAudioFormat to be 'mp3', got '%s'", cfg.DefaultAudioFormat)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  *DefaultConfig(),
			wantErr: false,
		},
		{
			name: "empty download path",
			config: Config{
				DownloadPath:           "",
				MaxConcurrentDownloads: 3,
				Port:                   8080,
			},
			wantErr: true,
		},
		{
			name: "invalid concurrent downloads",
			config: Config{
				DownloadPath:           "/tmp/test",
				MaxConcurrentDownloads: 0,
				Port:                   8080,
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			config: Config{
				DownloadPath:           "/tmp/test",
				MaxConcurrentDownloads: 3,
				Port:                   0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	originalConfig := DefaultConfig()
	originalConfig.Port = 9090
	originalConfig.MaxConcurrentDownloads = 5

	// Save config
	if err := originalConfig.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load config
	loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loadedConfig.Port != 9090 {
		t.Errorf("Expected Port to be 9090, got %d", loadedConfig.Port)
	}

	if loadedConfig.MaxConcurrentDownloads != 5 {
		t.Errorf("Expected MaxConcurrentDownloads to be 5, got %d", loadedConfig.MaxConcurrentDownloads)
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nonexistent.json")

	config, err := Load(configPath)
	if err != nil {
		t.Fatalf("Expected Load to create default config, got error: %v", err)
	}

	if config.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", config.Port)
	}

	// Check that file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be created")
	}
}
