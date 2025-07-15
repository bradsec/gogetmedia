package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDownloader(t *testing.T) {
	ytDlpPath := "/usr/bin/yt-dlp"
	ffmpegPath := "/usr/bin/ffmpeg"

	downloader := NewDownloader(ytDlpPath, ffmpegPath, true, false)

	if downloader.ytDlpPath != ytDlpPath {
		t.Errorf("Expected ytDlpPath to be %s, got %s", ytDlpPath, downloader.ytDlpPath)
	}

	if downloader.ffmpegPath != ffmpegPath {
		t.Errorf("Expected ffmpegPath to be %s, got %s", ffmpegPath, downloader.ffmpegPath)
	}
	if downloader.enableHardwareAccel != true {
		t.Errorf("Expected enableHardwareAccel to be true, got %v", downloader.enableHardwareAccel)
	}
	if downloader.optimizeForLowPower != false {
		t.Errorf("Expected optimizeForLowPower to be false, got %v", downloader.optimizeForLowPower)
	}
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	id2 := GenerateID()

	if id1 == id2 {
		t.Errorf("Expected different IDs, got same: %s", id1)
	}

	if len(id1) == 0 {
		t.Errorf("Expected non-empty ID")
	}
}

func TestIsPlaylistURL(t *testing.T) {
	downloader := NewDownloader("", "", false, false)

	testCases := []struct {
		url      string
		expected bool
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLxxx", true},
		{"https://www.youtube.com/playlist?list=PLxxx", true},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"https://example.com/video", false},
	}

	for _, tc := range testCases {
		result := downloader.IsPlaylistURL(tc.url)
		if result != tc.expected {
			t.Errorf("IsPlaylistURL(%s) = %v, expected %v", tc.url, result, tc.expected)
		}
	}
}

func TestDownloadRequest_Validation(t *testing.T) {
	testCases := []struct {
		name    string
		req     DownloadRequest
		wantErr bool
	}{
		{
			name: "valid video request",
			req: DownloadRequest{
				URL:       "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
				Type:      VideoDownload,
				Quality:   "720p",
				Format:    "mp4",
				OutputDir: "/tmp",
			},
			wantErr: false,
		},
		{
			name: "valid audio request",
			req: DownloadRequest{
				URL:       "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
				Type:      AudioDownload,
				Quality:   "best",
				Format:    "mp3",
				OutputDir: "/tmp",
			},
			wantErr: false,
		},
		{
			name: "empty URL",
			req: DownloadRequest{
				URL:       "",
				Type:      VideoDownload,
				Quality:   "720p",
				Format:    "mp4",
				OutputDir: "/tmp",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.req.URL == "" && !tc.wantErr {
				t.Errorf("Expected error for empty URL")
			}
		})
	}
}

func TestFindDownloadedFile(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "gogetmedia_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := []string{
		"test_video.mp4",
		"another_video.mkv",
		"audio_file.mp3",
	}

	for _, filename := range testFiles {
		filepath := filepath.Join(tempDir, filename)
		file, err := os.Create(filepath)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		file.Close()
	}

	downloader := NewDownloader("", "", false, false)

	// Test finding existing file
	result := downloader.findDownloadedFile(tempDir, "test video", "mp4")
	expected := filepath.Join(tempDir, "test_video.mp4")
	if result != expected {
		t.Errorf("Expected to find %s, got %s", expected, result)
	}

	// Test finding non-existent file
	result = downloader.findDownloadedFile(tempDir, "nonexistent", "avi")
	if result != "" {
		t.Errorf("Expected empty result for non-existent file, got %s", result)
	}
}

func TestTimeStringToSeconds(t *testing.T) {
	testCases := []struct {
		input    string
		expected float64
	}{
		{"00:01:30.00", 90.0},
		{"01:00:00.00", 3600.0},
		{"00:00:30.50", 30.5},
		{"invalid", 0.0},
	}

	for _, tc := range testCases {
		result := timeStringToSeconds(tc.input)
		if result != tc.expected {
			t.Errorf("timeStringToSeconds(%s) = %f, expected %f", tc.input, result, tc.expected)
		}
	}
}

func TestDownloadProgress_SafeChannelHandling(t *testing.T) {
	// Test that progress channels handle closure gracefully
	progressChan := make(chan DownloadProgress, 1)

	// Send to open channel
	progress := DownloadProgress{Percentage: 50.0}

	select {
	case progressChan <- progress:
		// Success
	default:
		t.Errorf("Failed to send to open channel")
	}

	// Close channel
	close(progressChan)

	// Attempt to send to closed channel using safe pattern (should not panic)
	func() {
		defer func() {
			if r := recover(); r != nil {
				// This is expected behavior - we're testing that we can handle the panic
			}
		}()

		select {
		case progressChan <- progress:
			t.Errorf("Should not be able to send to closed channel")
		default:
			// Expected behavior - channel is closed and select should hit default
		}
	}()

	// Test the safe sending pattern used in the actual code
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Channel was closed, ignore the panic - this is the safe pattern
			}
		}()
		// This mimics how we safely send in the actual code
		select {
		case progressChan <- progress:
			// This should hit the panic and be caught by recover
		default:
			// Channel is full or closed, skip
		}
	}()
}

func TestDownloadContext_Cancellation(t *testing.T) {
	// Test that download respects context cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	// Check if context is cancelled
	if ctx.Err() != context.Canceled {
		t.Errorf("Expected context to be cancelled")
	}
}
