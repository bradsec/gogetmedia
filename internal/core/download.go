package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DownloadType string

const (
	VideoDownload DownloadType = "video"
	AudioDownload DownloadType = "audio"
)

type DownloadStatus string

const (
	StatusQueued         DownloadStatus = "queued"
	StatusDownloading    DownloadStatus = "downloading"
	StatusPostProcessing DownloadStatus = "post-processing"
	StatusPaused         DownloadStatus = "paused"
	StatusCompleted      DownloadStatus = "completed"
	StatusFailed         DownloadStatus = "failed"
	StatusCancelled      DownloadStatus = "cancelled"
	StatusAlreadyExists  DownloadStatus = "already_exists"
)

type DownloadRequest struct {
	URL       string       `json:"url"`
	Type      DownloadType `json:"type"`
	Quality   string       `json:"quality"`
	Format    string       `json:"format"`
	OutputDir string       `json:"output_dir"`
}

type DownloadProgress struct {
	Percentage float64 `json:"percentage"`
	Speed      string  `json:"speed"`
	ETA        string  `json:"eta"`
	Size       string  `json:"size"`
}

type TitleUpdateCallback func(id, title string)
type StatusUpdateCallback func(id string, status DownloadStatus)

type Download struct {
	ID            string           `json:"id"`
	URL           string           `json:"url"`
	Type          DownloadType     `json:"type"`
	Quality       string           `json:"quality"`
	Format        string           `json:"format"`
	Status        DownloadStatus   `json:"status"`
	Progress      DownloadProgress `json:"progress"`
	Title         string           `json:"title"`
	Filename      string           `json:"filename"`
	OutputPath    string           `json:"output_path"`
	CreatedAt     time.Time        `json:"created_at"`
	CompletedAt   *time.Time       `json:"completed_at,omitempty"`
	Error         string           `json:"error,omitempty"`
	StatusMessage string           `json:"status_message,omitempty"`
}

type Downloader struct {
	ytDlpPath           string
	ffmpegPath          string
	enableHardwareAccel bool
	optimizeForLowPower bool
}

func NewDownloader(ytDlpPath, ffmpegPath string, enableHardwareAccel, optimizeForLowPower bool) *Downloader {
	return &Downloader{
		ytDlpPath:           ytDlpPath,
		ffmpegPath:          ffmpegPath,
		enableHardwareAccel: enableHardwareAccel,
		optimizeForLowPower: optimizeForLowPower,
	}
}

func (d *Downloader) Download(ctx context.Context, req DownloadRequest, progressChan chan<- DownloadProgress, titleCallback TitleUpdateCallback, statusCallback StatusUpdateCallback, downloadID string) (*Download, error) {
	download := &Download{
		ID:        downloadID, // Use the ID from the manager
		URL:       req.URL,
		Type:      req.Type,
		Quality:   req.Quality,
		Format:    req.Format,
		Status:    StatusQueued,
		CreatedAt: time.Now(),
	}

	log.Printf("[DOWNLOAD] %s: Added to queue - %s (%s %s)", download.ID, req.URL, req.Type, req.Format)

	// Try to get video info first (non-blocking)
	info, err := d.GetVideoInfo(req.URL)
	if err != nil {
		log.Printf("[DOWNLOAD] %s: Could not get video info, will extract during download", download.ID)
		// Don't fail the download - just use URL as fallback
		download.Title = req.URL
		download.Filename = fmt.Sprintf("download_%s", download.ID)
	} else {
		download.Title = info.Title
		download.Filename = SanitizeFilename(info.Title)
		log.Printf("[DOWNLOAD] %s: Title identified - %s", download.ID, info.Title)
	}

	// Update title in manager if callback is provided
	if titleCallback != nil {
		titleCallback(download.ID, download.Title)
	}

	// Send initial progress update with title information (with safe channel handling)
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Channel was closed or there was another panic, ignore it
				log.Printf("[DOWNLOAD] %s: Recovered from progress channel panic: %v", download.ID, r)
			}
		}()
		select {
		case progressChan <- DownloadProgress{
			Percentage: 0,
			Speed:      "",
			ETA:        "",
			Size:       "",
		}:
		case <-ctx.Done():
			// Context cancelled, don't send
		default:
			// Channel full, skip
		}
	}()

	// Set expected output extension based on type and format
	expectedExt := ""
	if req.Type == AudioDownload {
		expectedExt = "." + req.Format
	} else {
		expectedExt = "." + req.Format
	}

	// Ensure filename has correct extension
	if !strings.HasSuffix(download.Filename, expectedExt) {
		download.Filename = strings.TrimSuffix(download.Filename, filepath.Ext(download.Filename)) + expectedExt
	}

	download.OutputPath = filepath.Join(req.OutputDir, download.Filename)
	log.Printf("[DOWNLOAD] %s: Output path=%s", download.ID, download.OutputPath)

	// Build yt-dlp command
	args := d.buildYtDlpArgs(req, download)
	log.Printf("[DOWNLOAD] %s: yt-dlp command: %s %s", download.ID, d.ytDlpPath, strings.Join(args, " "))

	download.Status = StatusDownloading
	log.Printf("[DOWNLOAD] %s: Starting download", download.ID)
	log.Printf("[DOWNLOAD] %s: yt-dlp path: %s", download.ID, d.ytDlpPath)
	
	// Check if yt-dlp binary exists and is executable
	if _, err := os.Stat(d.ytDlpPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("yt-dlp binary not found at %s", d.ytDlpPath)
	}

	cmd := exec.CommandContext(ctx, d.ytDlpPath, args...)
	cmd.Dir = req.OutputDir

	// Set up process group for proper child process cleanup (platform-specific)
	setupProcessGroup(cmd, download.ID)

	log.Printf("[DOWNLOAD] %s: Working directory: %s", download.ID, req.OutputDir)

	// Create progress reader
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		download.Status = StatusFailed
		download.Error = fmt.Sprintf("Failed to create stdout pipe: %v", err)
		return download, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		download.Status = StatusFailed
		download.Error = fmt.Sprintf("Failed to create stderr pipe: %v", err)
		return download, err
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[DOWNLOAD] %s: Failed to start yt-dlp: %v", download.ID, err)
		download.Status = StatusFailed
		download.Error = fmt.Sprintf("Failed to start yt-dlp: %v", err)
		return download, err
	}

	log.Printf("[DOWNLOAD] %s: yt-dlp process started, PID: %d", download.ID, cmd.Process.Pid)

	// Monitor progress
	go d.monitorProgress(stdout, stderr, progressChan, statusCallback, download.ID)

	// Wait for completion
	log.Printf("[DOWNLOAD] %s: Waiting for yt-dlp to complete...", download.ID)
	err = cmd.Wait()

	if ctx.Err() == context.Canceled {
		log.Printf("[DOWNLOAD] %s: Download cancelled", download.ID)
		download.Status = StatusCancelled
		return download, nil
	}

	if err != nil {
		log.Printf("[DOWNLOAD] %s: yt-dlp failed: %v", download.ID, err)
		download.Status = StatusFailed

		// Enhanced error reporting
		errorMsg := d.categorizeError(err, download.URL)
		download.Error = errorMsg

		log.Printf("[DOWNLOAD] %s: Categorized error: %s", download.ID, errorMsg)
		return download, fmt.Errorf("download failed: %s", errorMsg)
	}

	log.Printf("[DOWNLOAD] %s: yt-dlp process completed", download.ID)

	// Find the actual downloaded file
	actualFilePath := d.findDownloadedFile(req.OutputDir, download.Title, req.Format)
	if actualFilePath != "" {
		download.OutputPath = actualFilePath
		download.Filename = filepath.Base(actualFilePath)

		// Extract actual title from filename if we used fallback URL
		if download.Title == req.URL {
			actualTitle := d.extractTitleFromFilename(download.Filename)
			if actualTitle != "" {
				download.Title = actualTitle
				// Update title in manager if callback is provided
				if titleCallback != nil {
					titleCallback(download.ID, actualTitle)
				}
			}
		}
	} else {
		log.Printf("[DOWNLOAD] %s: Warning - could not locate downloaded file in %s", download.ID, req.OutputDir)
	}

	download.Status = StatusCompleted
	now := time.Now()
	download.CompletedAt = &now
	log.Printf("[DOWNLOAD] %s: Download completed successfully - %s", download.ID, download.Title)

	return download, nil
}

// findDownloadedFile looks for the actual downloaded file based on title and format
func (d *Downloader) findDownloadedFile(outputDir, title, format string) string {
	// If title is a URL, we need to search for any file with the correct format
	if strings.HasPrefix(title, "http") {
		return d.findMostRecentFile(outputDir, format)
	}

	// Sanitize title for filename matching
	sanitizedTitle := SanitizeFilename(title)

	// List of possible filenames to check
	possibleFiles := []string{
		sanitizedTitle + "." + format,
		title + "." + format,
		// yt-dlp might create different variations
	}

	// Check each possible filename
	for _, filename := range possibleFiles {
		fullPath := filepath.Join(outputDir, filename)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}

	// If exact match not found, look for files with similar names
	files, err := os.ReadDir(outputDir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if filename contains sanitized title and has correct extension
		filename := file.Name()
		if strings.Contains(strings.ToLower(filename), strings.ToLower(sanitizedTitle)) &&
			strings.HasSuffix(strings.ToLower(filename), "."+format) {
			return filepath.Join(outputDir, filename)
		}
	}

	return ""
}

// findMostRecentFile finds the most recently created file with the given format
func (d *Downloader) findMostRecentFile(outputDir, format string) string {
	files, err := os.ReadDir(outputDir)
	if err != nil {
		return ""
	}

	var mostRecent os.FileInfo
	var mostRecentPath string

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		if strings.HasSuffix(strings.ToLower(filename), "."+format) {
			fullPath := filepath.Join(outputDir, filename)
			fileInfo, err := os.Stat(fullPath)
			if err != nil {
				continue
			}

			if mostRecent == nil || fileInfo.ModTime().After(mostRecent.ModTime()) {
				mostRecent = fileInfo
				mostRecentPath = fullPath
			}
		}
	}

	return mostRecentPath
}

// extractTitleFromFilename extracts the title from a filename by removing the extension
func (d *Downloader) extractTitleFromFilename(filename string) string {
	// Remove extension
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Basic cleanup - remove common yt-dlp artifacts
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.TrimSpace(title)

	return title
}

func (d *Downloader) buildYtDlpArgs(req DownloadRequest, download *Download) []string {
	args := []string{
		"--no-playlist",
		"--progress",
		"--newline",
		"--no-warnings",
		"--ignore-errors", // Continue on errors
		"--continue",      // Resume partial downloads if they exist
	}

	// Add Reddit-specific options if it's a Reddit URL
	if strings.Contains(req.URL, "reddit.com") || strings.Contains(req.URL, "redd.it") {
		args = append(args, "--extractor-args", "reddit:sort=best")
	}

	// Add ffmpeg path if configured and not default
	if d.ffmpegPath != "" && d.ffmpegPath != "ffmpeg" && d.ffmpegPath != "ffmpeg.exe" {
		args = append(args, "--ffmpeg-location", d.ffmpegPath)
	}

	if req.Type == AudioDownload {
		args = append(args, "--extract-audio")
		args = append(args, "--audio-format", req.Format)

		// Add FFmpeg post-processing for audio to ensure compatibility and quality
		audioFFmpegArgs := d.buildAudioFFmpegArgs(req.Format)
		if audioFFmpegArgs != "" {
			args = append(args, "--postprocessor-args", "ffmpeg:"+audioFFmpegArgs)
		}

		// Use yt-dlp's default filename template if we don't have a title
		if download.Title == req.URL {
			args = append(args, "--output", "%(title)s.%(ext)s")
		} else {
			args = append(args, "--output", download.Filename)
		}
	} else {
		// Video download - get best quality, then convert with ffmpeg if needed
		args = append(args, "--format", d.getVideoFormat(req.Quality, req.Format))

		// Add post-processing to ensure proper format with cross-platform compatible codecs
		ffmpegArgs := d.buildFFmpegArgs(req.Format)
		if ffmpegArgs != "" {
			args = append(args, "--merge-output-format", req.Format)
			args = append(args, "--postprocessor-args", "ffmpeg:"+ffmpegArgs)
		}

		// Use yt-dlp's default filename template if we don't have a title
		if download.Title == req.URL {
			args = append(args, "--output", "%(title)s.%(ext)s")
		} else {
			args = append(args, "--output", download.Filename)
		}
	}

	// Add URL
	args = append(args, req.URL)

	return args
}

// getHardwareAcceleration detects available hardware acceleration options
func (d *Downloader) getHardwareAcceleration() string {
	// Return empty if hardware acceleration is disabled
	if !d.enableHardwareAccel {
		return ""
	}
	
	// Check for NVIDIA NVENC support
	if d.checkHardwareSupport("h264_nvenc") {
		return "-hwaccel cuda"
	}
	
	// Check for Intel QuickSync support
	if d.checkHardwareSupport("h264_qsv") {
		return "-hwaccel qsv"
	}
	
	// Check for AMD/Intel VAAPI support (Linux)
	if d.checkHardwareSupport("h264_vaapi") {
		return "-hwaccel vaapi -hwaccel_device /dev/dri/renderD128"
	}
	
	// No hardware acceleration available
	return ""
}

// getHardwareEncoder returns the appropriate hardware encoder
func (d *Downloader) getHardwareEncoder() string {
	// Check for NVIDIA NVENC support
	if d.checkHardwareSupport("h264_nvenc") {
		return "h264_nvenc"
	}
	
	// Check for Intel QuickSync support  
	if d.checkHardwareSupport("h264_qsv") {
		return "h264_qsv"
	}
	
	// Check for AMD/Intel VAAPI support (Linux)
	if d.checkHardwareSupport("h264_vaapi") {
		return "h264_vaapi"
	}
	
	// Fallback to software encoder
	return "libx264"
}

// checkHardwareSupport checks if a hardware encoder is available
func (d *Downloader) checkHardwareSupport(encoder string) bool {
	cmd := exec.Command(d.ffmpegPath, "-encoders")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	return strings.Contains(string(output), encoder)
}

// buildAudioFFmpegArgs creates optimized FFmpeg arguments for audio processing
// RequiresFfmpeg checks if the given download type and format require ffmpeg for post-processing
func RequiresFfmpeg(downloadType DownloadType, format string) bool {
	switch downloadType {
	case AudioDownload:
		// All audio formats require ffmpeg for post-processing
		return true
	case VideoDownload:
		// Video formats that require ffmpeg for post-processing/merging
		switch format {
		case "mp4", "mkv", "webm", "avi":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func (d *Downloader) buildAudioFFmpegArgs(format string) string {
	baseArgs := "-progress pipe:2 -nostats -loglevel error"

	switch format {
	case "mp3":
		// Optimized MP3 - good quality, faster encoding
		// VBR quality 2 (very good quality), standard sample rate
		return fmt.Sprintf("-c:a libmp3lame -q:a 2 -ac 2 -ar 44100 %s", baseArgs)

	case "m4a":
		// Optimized AAC - good quality, faster encoding
		// Lower bitrate but still good quality, standard sample rate
		return fmt.Sprintf("-c:a aac -b:a 128k -ac 2 -ar 44100 %s", baseArgs)

	case "wav":
		// Standard WAV - no unnecessary processing
		// 16-bit PCM, standard sample rate
		return fmt.Sprintf("-c:a pcm_s16le -ac 2 -ar 44100 %s", baseArgs)

	case "flac":
		// Faster FLAC encoding - lower compression for speed
		// Standard sample rate, compression level 3 (faster)
		return fmt.Sprintf("-c:a flac -compression_level 3 -ac 2 -ar 44100 %s", baseArgs)

	default:
		// Fallback to MP3 for unknown formats
		return fmt.Sprintf("-c:a libmp3lame -q:a 2 -ac 2 -ar 44100 %s", baseArgs)
	}
}

// buildFFmpegArgs creates optimized FFmpeg arguments for cross-platform compatibility
func (d *Downloader) buildFFmpegArgs(format string) string {
	baseArgs := "-progress pipe:2 -nostats -loglevel error"
	
	// Check for hardware acceleration support
	hwAccel := d.getHardwareAcceleration()

	switch format {
	case "mp4":
		// Optimized H.264 + AAC - good quality, faster encoding
		// CRF 23 = balanced quality, preset fast = better speed
		// Main profile for good compatibility, standard audio settings
		// movflags +faststart = optimizes for streaming/web playback
		if hwAccel != "" {
			return fmt.Sprintf("%s -c:v %s -crf 23 -preset fast -profile:v main -level:v 4.0 -pix_fmt yuv420p -c:a aac -b:a 128k -ac 2 -ar 44100 -movflags +faststart %s", hwAccel, d.getHardwareEncoder(), baseArgs)
		}
		return fmt.Sprintf("-c:v libx264 -crf 23 -preset fast -profile:v main -level:v 4.0 -pix_fmt yuv420p -c:a aac -b:a 128k -ac 2 -ar 44100 -movflags +faststart %s", baseArgs)

	case "mkv":
		// Optimized H.264 + AAC in MKV container - faster encoding
		// Same optimized settings as MP4 for consistency
		return fmt.Sprintf("-c:v libx264 -crf 23 -preset fast -profile:v main -level:v 4.0 -pix_fmt yuv420p -c:a aac -b:a 128k -ac 2 -ar 44100 %s", baseArgs)

	case "webm":
		// Optimized VP9 + Opus - faster encoding for low-power PCs
		// CRF 30 for faster encoding, speed 4 = much faster
		// Reduced threading overhead, simpler settings
		return fmt.Sprintf("-c:v libvpx-vp9 -crf 30 -b:v 0 -speed 4 -threads 2 -c:a libopus -b:a 128k -ac 2 -ar 44100 %s", baseArgs)

	case "avi":
		// Optimized H.264 + AAC in AVI (for legacy compatibility)
		// Use same optimized settings as MP4 but without faststart flag
		return fmt.Sprintf("-c:v libx264 -crf 23 -preset fast -profile:v main -level:v 4.0 -pix_fmt yuv420p -c:a aac -b:a 128k -ac 2 -ar 44100 %s", baseArgs)

	default:
		// Fallback for any other format - use optimized H.264 + AAC
		return fmt.Sprintf("-c:v libx264 -crf 23 -preset fast -pix_fmt yuv420p -c:a aac -b:a 128k -ac 2 -ar 44100 %s", baseArgs)
	}
}

func (d *Downloader) getVideoFormat(quality, format string) string {
	// For low power optimization, prefer native formats to avoid conversion
	codecFilter := ""
	if d.optimizeForLowPower {
		switch format {
		case "mp4":
			codecFilter = "[ext=mp4]/[vcodec^=avc1]/[vcodec^=h264]"
		case "webm":
			codecFilter = "[ext=webm]/[vcodec^=vp9]/[vcodec^=vp8]"
		case "mkv":
			codecFilter = "[ext=mkv]/[vcodec^=h264]"
		}
	}
	
	switch quality {
	case "best":
		// Get absolute best quality available - no format restrictions
		if codecFilter != "" {
			return fmt.Sprintf("bestvideo%s+bestaudio/best%s/bestvideo+bestaudio/best", codecFilter, codecFilter)
		}
		return "bestvideo+bestaudio/best"
	case "worst":
		// Get absolute best quality available with preferred codecs
		if codecFilter != "" {
			return fmt.Sprintf("worstvideo%s+worstaudio/worst%s/worstvideo+worstaudio/worst", codecFilter, codecFilter)
		}
		return "worstvideo+worstaudio/worst"
	case "4K":
		if codecFilter != "" {
			return fmt.Sprintf("bestvideo[height<=2160]%s+bestaudio/best[height<=2160]%s/bestvideo[height<=2160]+bestaudio/best[height<=2160]", codecFilter, codecFilter)
		}
		return "bestvideo[height<=2160]+bestaudio/best[height<=2160]"
	case "2K":
		if codecFilter != "" {
			return fmt.Sprintf("bestvideo[height<=1440]%s+bestaudio/best[height<=1440]%s/bestvideo[height<=1440]+bestaudio/best[height<=1440]", codecFilter, codecFilter)
		}
		return "bestvideo[height<=1440]+bestaudio/best[height<=1440]"
	case "1080p":
		if codecFilter != "" {
			return fmt.Sprintf("bestvideo[height<=1080]%s+bestaudio/best[height<=1080]%s/bestvideo[height<=1080]+bestaudio/best[height<=1080]", codecFilter, codecFilter)
		}
		return "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
	case "720p":
		if codecFilter != "" {
			return fmt.Sprintf("bestvideo[height<=720]%s+bestaudio/best[height<=720]%s/bestvideo[height<=720]+bestaudio/best[height<=720]", codecFilter, codecFilter)
		}
		return "bestvideo[height<=720]+bestaudio/best[height<=720]"
	case "480p":
		if codecFilter != "" {
			return fmt.Sprintf("bestvideo[height<=480]%s+bestaudio/best[height<=480]%s/bestvideo[height<=480]+bestaudio/best[height<=480]", codecFilter, codecFilter)
		}
		return "bestvideo[height<=480]+bestaudio/best[height<=480]"
	case "360p":
		if codecFilter != "" {
			return fmt.Sprintf("bestvideo[height<=360]%s+bestaudio/best[height<=360]%s/bestvideo[height<=360]+bestaudio/best[height<=360]", codecFilter, codecFilter)
		}
		return "bestvideo[height<=360]+bestaudio/best[height<=360]"
	default:
		// Handle any other specific resolutions
		if strings.HasSuffix(quality, "p") {
			height := strings.TrimSuffix(quality, "p")
			if codecFilter != "" {
				return fmt.Sprintf("bestvideo[height<=%s]%s+bestaudio/best[height<=%s]%s/bestvideo[height<=%s]+bestaudio/best[height<=%s]", height, codecFilter, height, codecFilter, height, height)
			}
			return fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best[height<=%s]", height, height)
		}
		if codecFilter != "" {
			return fmt.Sprintf("bestvideo%s+bestaudio/best%s/bestvideo+bestaudio/best", codecFilter, codecFilter)
		}
		return "bestvideo+bestaudio/best"
	}
}

// Helper function to get video duration in seconds
func (d *Downloader) getVideoDuration(videoPath string) float64 {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[FFMPEG] Failed to get video duration: %v", err)
		return 0
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		log.Printf("[FFMPEG] Failed to parse duration: %v", err)
		return 0
	}

	return duration
}

// categorizeError provides user-friendly error messages based on the error type
func (d *Downloader) categorizeError(err error, url string) string {
	errStr := strings.ToLower(err.Error())

	// Check for common error patterns
	switch {
	case strings.Contains(errStr, "video unavailable"):
		return "Video is unavailable or has been removed"
	case strings.Contains(errStr, "private video"):
		return "Video is private and cannot be downloaded"
	case strings.Contains(errStr, "age-restricted"):
		return "Video is age-restricted and requires authentication"
	case strings.Contains(errStr, "region blocked") || strings.Contains(errStr, "not available in your country"):
		return "Video is not available in your region"
	case strings.Contains(errStr, "copyright"):
		return "Video is blocked due to copyright restrictions"
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection"):
		return "Network connection issue - please check your internet connection"
	case strings.Contains(errStr, "timeout"):
		return "Download timed out - the server may be slow or overloaded"
	case strings.Contains(errStr, "unsupported url"):
		return "This website or URL format is not supported"
	case strings.Contains(errStr, "format not available"):
		return "Requested quality or format is not available for this video"
	case strings.Contains(errStr, "login") || strings.Contains(errStr, "authentication"):
		return "Video requires login or authentication"
	case strings.Contains(errStr, "quota exceeded"):
		return "API quota exceeded - please try again later"
	case strings.Contains(errStr, "too many requests"):
		return "Too many requests - please wait and try again"
	case strings.Contains(errStr, "disk") || strings.Contains(errStr, "space"):
		return "Insufficient disk space to complete download"
	case strings.Contains(errStr, "permission denied"):
		return "Permission denied - check file/directory permissions"
	case strings.Contains(errStr, "file exists"):
		return "File already exists at destination"
	case strings.Contains(errStr, "no such file"):
		return "Required file or directory not found"
	case strings.Contains(errStr, "executable file not found"):
		return "yt-dlp or ffmpeg executable not found - please check installation"
	case strings.Contains(errStr, "extract") && strings.Contains(errStr, "info"):
		return "Failed to extract video information - video may be corrupted or unavailable"
	case strings.Contains(errStr, "ffmpeg"):
		return "FFmpeg processing failed - video conversion error"
	case strings.Contains(errStr, "postprocessing"):
		return "Post-processing failed - video downloaded but conversion failed"
	case strings.Contains(errStr, "http") && (strings.Contains(errStr, "404") || strings.Contains(errStr, "not found")):
		return "Video not found (404 error)"
	case strings.Contains(errStr, "http") && (strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden")):
		return "Access forbidden (403 error) - video may require authentication"
	case strings.Contains(errStr, "http") && (strings.Contains(errStr, "500") || strings.Contains(errStr, "internal server")):
		return "Server error (500) - please try again later"
	case strings.Contains(errStr, "http") && strings.Contains(errStr, "503"):
		return "Service temporarily unavailable (503) - please try again later"
	case strings.Contains(errStr, "killed") || strings.Contains(errStr, "terminated"):
		return "Download was cancelled or interrupted"
	case strings.Contains(errStr, "context canceled"):
		return "Download was cancelled by user"
	case strings.Contains(errStr, "context deadline exceeded"):
		return "Download timed out"
	default:
		// If no specific pattern matches, provide the original error with some cleanup
		if len(errStr) > 200 {
			return fmt.Sprintf("Download failed: %s...", errStr[:200])
		}
		return fmt.Sprintf("Download failed: %s", err.Error())
	}
}

// Helper function to convert time string (HH:MM:SS.mmm) to seconds
func timeStringToSeconds(timeStr string) float64 {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 3 {
		return 0
	}

	hours, _ := strconv.ParseFloat(parts[0], 64)
	minutes, _ := strconv.ParseFloat(parts[1], 64)
	seconds, _ := strconv.ParseFloat(parts[2], 64)

	return hours*3600 + minutes*60 + seconds
}

func (d *Downloader) monitorProgress(stdout, stderr io.ReadCloser, progressChan chan<- DownloadProgress, statusCallback StatusUpdateCallback, downloadID string) {
	// Multiple regex patterns to match different yt-dlp output formats
	progressRegexes := []*regexp.Regexp{
		// [download]   0.0% of   11.21MiB at    2.47MiB/s ETA 00:04
		regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+(\S+)\s+at\s+(\S+)\s+ETA\s+(\S+)`),
		// [download]   0.0% of ~  11.21MiB at    2.47MiB/s ETA 00:04
		regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+~\s+(\S+)\s+at\s+(\S+)\s+ETA\s+(\S+)`),
		// [download]   0.0% of unknown size at    2.47MiB/s ETA Unknown
		regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+(.+?)\s+at\s+(\S+)\s+ETA\s+(\S+)`),
		// [download] 100% of   11.21MiB in 00:04
		regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%\s+of\s+(\S+)\s+in\s+(\S+)`),
	}

	// Post-processing regex patterns
	postProcessRegexes := []*regexp.Regexp{
		regexp.MustCompile(`\[ffmpeg\]`),
		regexp.MustCompile(`\[Merger\]`),
		regexp.MustCompile(`Merging formats into`),
		regexp.MustCompile(`\[post-processor\]`),
	}

	// Keep only the regex patterns we actually use
	ffmpegProgressRegex := regexp.MustCompile(`frame=\s*(\d+).*?time=(\d{2}:\d{2}:\d{2}\.\d{2}).*?bitrate=\s*([\d\.]+)kbits/s`)
	ffmpegProgressFrameRegex := regexp.MustCompile(`^frame=(\d+)$`)
	ffmpegProgressTimeRegex := regexp.MustCompile(`^out_time_ms=(\d+)$`)
	ffmpegProgressBitrateRegex := regexp.MustCompile(`^bitrate=(\d+\.?\d*)kbits/s$`)
	ffmpegProgressSpeedRegex := regexp.MustCompile(`^speed=(\d+\.?\d*)x$`)

	// Regex to extract duration from FFmpeg output
	durationRegex := regexp.MustCompile(`Duration:\s*([\d:\.]+)`)

	lastPercentage := -1.0
	isPostProcessing := false

	// FFmpeg progress tracking variables (protected by mutex for thread safety)
	var progressMutex sync.RWMutex
	var ffmpegFrames int64
	var ffmpegDuration, ffmpegCurrentTime float64
	var ffmpegBitrate string
	var ffmpegSpeed string

	// Helper function to safely update FFmpeg progress
	updateFFmpegProgress := func(frames int64, duration, currentTime float64, bitrate, speed string) {
		progressMutex.Lock()
		defer progressMutex.Unlock()
		if frames > 0 {
			ffmpegFrames = frames
		}
		if duration > 0 {
			ffmpegDuration = duration
		}
		if currentTime > 0 {
			ffmpegCurrentTime = currentTime
		}
		if bitrate != "" {
			ffmpegBitrate = bitrate
		}
		if speed != "" {
			ffmpegSpeed = speed
		}
	}

	// Helper function to safely read FFmpeg progress
	readFFmpegProgress := func() (int64, float64, float64, string, string) {
		progressMutex.RLock()
		defer progressMutex.RUnlock()
		return ffmpegFrames, ffmpegDuration, ffmpegCurrentTime, ffmpegBitrate, ffmpegSpeed
	}

	// Monitor both stdout and stderr for progress
	stderrScanner := bufio.NewScanner(stderr)

	// Start stderr monitoring in a separate goroutine
	go func() {
		for stderrScanner.Scan() {
			line := stderrScanner.Text()

			// Log stderr line for debugging
			if strings.Contains(line, "frame=") || strings.Contains(line, "time=") || strings.Contains(line, "bitrate=") {
				log.Printf("[DOWNLOAD] %s: FFmpeg stderr: %s", downloadID, line)
			}

			// Parse FFmpeg progress from stderr as well
			if isPostProcessing {
				progressUpdated := false
				var frames int64
				var currentTime float64
				var bitrate, speed string

				// First try to match key=value format from -progress pipe:2
				if matches := ffmpegProgressFrameRegex.FindStringSubmatch(line); matches != nil {
					frames, _ = strconv.ParseInt(matches[1], 10, 64)
					updateFFmpegProgress(frames, 0, 0, "", "")
					progressUpdated = true
					log.Printf("[DOWNLOAD] %s: FFmpeg stderr frame: %d", downloadID, frames)
				} else if matches := ffmpegProgressTimeRegex.FindStringSubmatch(line); matches != nil {
					// out_time_ms is in microseconds, convert to seconds
					timeMs, _ := strconv.ParseInt(matches[1], 10, 64)
					currentTime = float64(timeMs) / 1000000.0
					updateFFmpegProgress(0, 0, currentTime, "", "")
					progressUpdated = true
					log.Printf("[DOWNLOAD] %s: FFmpeg stderr time: %.2fs", downloadID, currentTime)
				} else if matches := ffmpegProgressBitrateRegex.FindStringSubmatch(line); matches != nil {
					bitrate = matches[1] + "kbps"
					updateFFmpegProgress(0, 0, 0, bitrate, "")
					progressUpdated = true
					log.Printf("[DOWNLOAD] %s: FFmpeg stderr bitrate: %s", downloadID, bitrate)
				} else if matches := ffmpegProgressSpeedRegex.FindStringSubmatch(line); matches != nil {
					speed = matches[1] + "x"
					updateFFmpegProgress(0, 0, 0, "", speed)
					progressUpdated = true
					log.Printf("[DOWNLOAD] %s: FFmpeg stderr speed: %s", downloadID, speed)
				} else if matches := ffmpegProgressRegex.FindStringSubmatch(line); matches != nil {
					// Fallback to standard FFmpeg output format
					frames, _ = strconv.ParseInt(matches[1], 10, 64)
					currentTime = timeStringToSeconds(matches[2])
					bitrate = matches[3] + "kbps"
					updateFFmpegProgress(frames, 0, currentTime, bitrate, "")
					progressUpdated = true
					log.Printf("[DOWNLOAD] %s: FFmpeg stderr progress - frame=%d, time=%.2fs, bitrate=%s", downloadID, frames, currentTime, bitrate)
				}

				// Calculate and send progress if updated
				if progressUpdated {
					frames, duration, currentTime, bitrate, speed := readFFmpegProgress()
					if duration > 0 && currentTime > 0 {
						progressPercent := (currentTime / duration) * 100
						if progressPercent > 100 {
							progressPercent = 100
						}

						// Estimate remaining time
						eta := ""
						if progressPercent > 0 && progressPercent < 100 {
							remainingTime := (duration - currentTime)
							if remainingTime > 0 {
								eta = fmt.Sprintf("%.0fs", remainingTime)
							}
						}

						// Use speed if available, otherwise bitrate
						speedInfo := bitrate
						if speed != "" {
							speedInfo = speed
						}

						log.Printf("[DOWNLOAD] %s: FFmpeg stderr progress calculated - %.1f%% (%.2fs/%.2fs)", downloadID, progressPercent, currentTime, duration)

						// Send FFmpeg progress update
						func() {
							defer func() {
								if r := recover(); r != nil {
									// Channel was closed, ignore
								}
							}()
							select {
							case progressChan <- DownloadProgress{
								Percentage: progressPercent,
								Speed:      speedInfo,
								ETA:        eta,
								Size:       fmt.Sprintf("Frame %d", frames),
							}:
							default:
								// Channel is full, skip
							}
						}()
					}
				}
			}
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		// Extract duration from FFmpeg output if we're in post-processing
		if isPostProcessing {
			_, duration, _, _, _ := readFFmpegProgress()
			if duration == 0 {
				if matches := durationRegex.FindStringSubmatch(line); matches != nil {
					detectedDuration := timeStringToSeconds(matches[1])
					updateFFmpegProgress(0, detectedDuration, 0, "", "")
					log.Printf("[DOWNLOAD] %s: Detected video duration: %.2fs", downloadID, detectedDuration)

					// Send updated progress with duration info
					func() {
						defer func() {
							if r := recover(); r != nil {
								// Channel was closed, ignore
							}
						}()
						select {
						case progressChan <- DownloadProgress{
							Percentage: 0,
							Speed:      "FFmpeg starting...",
							ETA:        fmt.Sprintf("Duration: %.0fs", detectedDuration),
							Size:       "Processing",
						}:
						default:
							// Channel is full, skip
						}
					}()
				}
			}
		}

		// Check for post-processing indicators
		for _, postRegex := range postProcessRegexes {
			if postRegex.MatchString(line) && !isPostProcessing {
				isPostProcessing = true
				log.Printf("[DOWNLOAD] %s: Starting post-processing with ffmpeg", downloadID)

				// Update status to post-processing
				if statusCallback != nil {
					statusCallback(downloadID, StatusPostProcessing)
				}

				// Send a simple status update to indicate post-processing
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Channel was closed, ignore
						}
					}()
					select {
					case progressChan <- DownloadProgress{
						Percentage: 0, // No percentage during post-processing
						Speed:      "Converting with FFmpeg",
						ETA:        "Processing",
						Size:       "Converting",
					}:
					default:
						// Channel is full, skip
					}
				}()

				// Note: Post-processing progress will be updated by the fallback mechanism below

				break
			}
		}

		// Simple FFmpeg status during post-processing - no progress parsing
		if isPostProcessing {
			// Just log FFmpeg activity without trying to parse progress
			if strings.Contains(line, "frame=") || strings.Contains(line, "time=") || strings.Contains(line, "bitrate=") {
				log.Printf("[DOWNLOAD] %s: FFmpeg processing: %s", downloadID, strings.TrimSpace(line))
			}
		}

		// Try each regex pattern for progress
		for _, progressRegex := range progressRegexes {
			if matches := progressRegex.FindStringSubmatch(line); matches != nil {
				percentage, _ := strconv.ParseFloat(matches[1], 64)

				// Only log progress at 25% intervals to reduce noise
				if percentage >= lastPercentage+25 || percentage == 100 {
					log.Printf("[DOWNLOAD] %s: Progress %.0f%%", downloadID, percentage)
					lastPercentage = percentage
				}

				progress := DownloadProgress{
					Percentage: percentage,
					Size:       matches[2],
				}

				// Handle different match groups for speed and ETA
				if len(matches) >= 4 {
					progress.Speed = matches[3]
				}
				if len(matches) >= 5 {
					progress.ETA = matches[4]
				}

				// Safe channel send - check if channel is closed
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Channel was closed, ignore the panic
						}
					}()
					select {
					case progressChan <- progress:
					default:
						// Channel is full, skip this update
					}
				}()
				break // Exit regex loop once we find a match
			}
		}
	}

	// Also read from stderr for error messages
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Only log actual errors, not all stderr output
			if strings.Contains(strings.ToLower(line), "error") ||
				strings.Contains(strings.ToLower(line), "warning") ||
				strings.Contains(strings.ToLower(line), "failed") {
				log.Printf("[DOWNLOAD] %s: %s", downloadID, line)
			}
		}
	}()
}

type VideoInfo struct {
	Title    string
	Filename string
}

type PlaylistItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Duration string `json:"duration"`
}

func (d *Downloader) GetVideoInfo(url string) (*VideoInfo, error) {
	// First validate the URL by trying to extract info with a timeout
	log.Printf("[INFO] Getting video info for URL: %s", url)

	// Create a context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, d.ytDlpPath, "--get-title", "--get-filename", "--no-warnings", "--no-playlist", url)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[INFO] Failed to get video info for %s: %v", url, err)
		// Check if it's a timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout getting video info for URL: %s", url)
		}
		// Check if it's a URL validation error
		if strings.Contains(err.Error(), "exit status") {
			return nil, fmt.Errorf("invalid URL or unsupported site: %s", url)
		}
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	log.Printf("[INFO] yt-dlp output lines: %v", lines)
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected output format from yt-dlp")
	}

	info := &VideoInfo{
		Title:    lines[0],
		Filename: lines[1],
	}
	log.Printf("[INFO] Video info retrieved: Title=%s, Filename=%s", info.Title, info.Filename)
	return info, nil
}

func GenerateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// IsPlaylistURL checks if a URL is a playlist
func (d *Downloader) IsPlaylistURL(url string) bool {
	// Check for common playlist URL patterns
	return strings.Contains(url, "list=") || strings.Contains(url, "playlist?")
}

// GetPlaylistItems extracts all items from a playlist
func (d *Downloader) GetPlaylistItems(url string) ([]PlaylistItem, error) {
	log.Printf("[PLAYLIST] Getting playlist items for URL: %s", url)

	// Create a context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use yt-dlp to extract playlist info in JSON format
	cmd := exec.CommandContext(ctx, d.ytDlpPath, "--flat-playlist", "--dump-json", "--no-warnings", url)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("[PLAYLIST] Failed to get playlist items for %s: %v", url, err)
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timeout getting playlist items for URL: %s", url)
		}
		return nil, fmt.Errorf("failed to get playlist items: %w", err)
	}

	var items []PlaylistItem
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			URL      string `json:"url"`
			Duration string `json:"duration_string"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			log.Printf("[PLAYLIST] Failed to parse JSON line: %s, error: %v", line, err)
			continue
		}

		items = append(items, PlaylistItem{
			ID:       entry.ID,
			Title:    entry.Title,
			URL:      entry.URL,
			Duration: entry.Duration,
		})
	}

	log.Printf("[PLAYLIST] Found %d items in playlist", len(items))
	return items, nil
}
