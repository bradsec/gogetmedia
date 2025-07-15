package manager

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gogetmedia/internal/config"
	"gogetmedia/internal/core"
)

type DownloadManager struct {
	downloader       *core.Downloader
	downloads        map[string]*core.Download
	queue            chan *core.Download
	maxConcurrent    int
	activeWorkers    int             // Track active workers
	workerCtx        context.Context // Separate context for workers
	workerCancel     context.CancelFunc
	progressChannels map[string]chan core.DownloadProgress
	cancelFuncs      map[string]context.CancelFunc
	pausedDownloads  map[string]*core.Download
	processingUrls   map[string]bool // Track URLs currently being processed
	mutex            sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	outputDir        string
	config           *config.Config
}

func NewDownloadManager(downloader *core.Downloader, maxConcurrent int, outputDir string, cfg *config.Config) *DownloadManager {
	ctx, cancel := context.WithCancel(context.Background())
	workerCtx, workerCancel := context.WithCancel(ctx)

	dm := &DownloadManager{
		downloader:       downloader,
		downloads:        make(map[string]*core.Download),
		queue:            make(chan *core.Download, 100),
		maxConcurrent:    maxConcurrent,
		activeWorkers:    0,
		workerCtx:        workerCtx,
		workerCancel:     workerCancel,
		progressChannels: make(map[string]chan core.DownloadProgress),
		cancelFuncs:      make(map[string]context.CancelFunc),
		pausedDownloads:  make(map[string]*core.Download),
		processingUrls:   make(map[string]bool),
		ctx:              ctx,
		cancel:           cancel,
		outputDir:        outputDir,
		config:           cfg,
	}

	// Start workers
	dm.startWorkers(maxConcurrent)

	// Load previous state
	if err := dm.LoadState(); err != nil {
		log.Printf("[MANAGER] Failed to load previous state: %v", err)
	}

	// Start cleanup worker if auto-expiry is enabled
	if cfg.CompletedFileExpiryHours > 0 {
		go dm.cleanupWorker()
	}

	// Start periodic state saving
	dm.StartPeriodicStateSave()

	return dm
}

func (dm *DownloadManager) AddDownload(req core.DownloadRequest) (*core.Download, error) {
	// Check if URL is a playlist - don't auto-process playlists
	if dm.downloader.IsPlaylistURL(req.URL) {
		return nil, fmt.Errorf("playlist URL detected - use playlist-specific endpoints instead")
	}

	// Check if this URL is already being processed
	dm.mutex.Lock()
	if dm.processingUrls[req.URL] {
		dm.mutex.Unlock()
		return nil, fmt.Errorf("this URL is already being processed")
	}

	// Check if this URL with same type/quality/format is already present
	for _, download := range dm.downloads {
		if download.URL == req.URL &&
			download.Type == req.Type &&
			download.Quality == req.Quality &&
			download.Format == req.Format {

			// Provide specific error message based on status
			switch download.Status {
			case core.StatusQueued, core.StatusDownloading, core.StatusPostProcessing:
				dm.mutex.Unlock()
				return nil, fmt.Errorf("this URL is already being downloaded with the same quality and format")
			case core.StatusCompleted, core.StatusAlreadyExists:
				dm.mutex.Unlock()
				return nil, fmt.Errorf("this URL has already been downloaded with the same quality and format")
			case core.StatusFailed:
				dm.mutex.Unlock()
				return nil, fmt.Errorf("this URL was previously attempted with the same settings. Remove the failed download first to retry")
			}
		}
	}

	// Mark URL as being processed
	dm.processingUrls[req.URL] = true
	dm.mutex.Unlock()

	download := &core.Download{
		ID:        core.GenerateID(),
		URL:       req.URL,
		Type:      req.Type,
		Quality:   req.Quality,
		Format:    req.Format,
		Status:    core.StatusQueued,
		CreatedAt: time.Now(),
	}

	log.Printf("[MANAGER] Adding download %s to queue: URL=%s, Type=%s", download.ID, req.URL, req.Type)

	// Check if file already exists
	if existingFile := dm.checkFileExists(req); existingFile != "" {
		log.Printf("[MANAGER] Download %s: File already exists at %s", download.ID, existingFile)
		download.Status = core.StatusAlreadyExists
		download.OutputPath = existingFile
		download.Filename = filepath.Base(existingFile)
		now := time.Now()
		download.CompletedAt = &now

		// Try to extract title from filename
		download.Title = dm.extractTitleFromPath(existingFile)

		dm.mutex.Lock()
		dm.downloads[download.ID] = download
		dm.progressChannels[download.ID] = make(chan core.DownloadProgress, 10)
		// Clean up processing URL since file already exists
		delete(dm.processingUrls, req.URL)
		dm.mutex.Unlock()

		return download, nil
	}

	dm.mutex.Lock()
	dm.downloads[download.ID] = download
	dm.progressChannels[download.ID] = make(chan core.DownloadProgress, 10)
	dm.mutex.Unlock()

	// Add to queue
	select {
	case dm.queue <- download:
		log.Printf("[MANAGER] Download %s added to queue successfully", download.ID)
		return download, nil
	default:
		log.Printf("[MANAGER] Download queue is full, rejecting download %s", download.ID)
		dm.mutex.Lock()
		delete(dm.downloads, download.ID)
		delete(dm.progressChannels, download.ID)
		delete(dm.processingUrls, req.URL)
		dm.mutex.Unlock()
		return nil, fmt.Errorf("download queue is full")
	}
}

func (dm *DownloadManager) AddPlaylistDownload(req core.DownloadRequest) (*core.Download, error) {
	log.Printf("[MANAGER] Processing playlist URL: %s", req.URL)

	// Get playlist items
	items, err := dm.downloader.GetPlaylistItems(req.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist items: %w", err)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no items found in playlist")
	}

	log.Printf("[MANAGER] Found %d items in playlist, creating individual downloads", len(items))

	// Create a download for each playlist item
	var firstDownload *core.Download
	for i, item := range items {
		download := &core.Download{
			ID:        core.GenerateID(),
			URL:       fmt.Sprintf("https://www.youtube.com/watch?v=%s", item.ID),
			Type:      req.Type,
			Quality:   req.Quality,
			Format:    req.Format,
			Status:    core.StatusQueued,
			Title:     item.Title,
			CreatedAt: time.Now(),
		}

		dm.mutex.Lock()
		dm.downloads[download.ID] = download
		dm.progressChannels[download.ID] = make(chan core.DownloadProgress, 10)
		dm.mutex.Unlock()

		// Add to queue
		select {
		case dm.queue <- download:
			log.Printf("[MANAGER] Playlist item %d/%d added to queue: %s", i+1, len(items), item.Title)
			if firstDownload == nil {
				firstDownload = download
			}
		default:
			log.Printf("[MANAGER] Download queue is full, skipping remaining playlist items")
			break
		}
	}

	return firstDownload, nil
}

func (dm *DownloadManager) GetDownload(id string) (*core.Download, bool) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	download, exists := dm.downloads[id]
	return download, exists
}

func (dm *DownloadManager) GetAllDownloads() []*core.Download {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	downloads := make([]*core.Download, 0, len(dm.downloads))
	for _, download := range dm.downloads {
		downloads = append(downloads, download)
	}

	return downloads
}

func (dm *DownloadManager) CancelDownload(id string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	download, exists := dm.downloads[id]
	if !exists {
		return fmt.Errorf("download not found")
	}

	if download.Status == core.StatusDownloading || download.Status == core.StatusPostProcessing {
		// Cancel the actual download or post-processing process
		if cancelFunc, exists := dm.cancelFuncs[id]; exists {
			log.Printf("[MANAGER] Cancelling %s process for download %s", download.Status, id)
			cancelFunc()
			delete(dm.cancelFuncs, id)
		}
		
		// Clean up temporary files immediately when cancelling post-processing
		if download.Status == core.StatusPostProcessing {
			log.Printf("[MANAGER] Cleaning up temporary files for cancelled post-processing download %s", id)
			dm.cleanupTemporaryFiles(download)
		}
		
		download.Status = core.StatusCancelled
	} else if download.Status == core.StatusQueued {
		download.Status = core.StatusCancelled
	}

	// Clean up processing URL on cancellation
	delete(dm.processingUrls, download.URL)

	return nil
}

func (dm *DownloadManager) PauseDownload(id string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	download, exists := dm.downloads[id]
	if !exists {
		return fmt.Errorf("download not found")
	}

	if download.Status == core.StatusDownloading {
		// Cancel the actual download process (this will stop yt-dlp)
		if cancelFunc, exists := dm.cancelFuncs[id]; exists {
			cancelFunc()
			delete(dm.cancelFuncs, id)
		}
		download.Status = core.StatusPaused
		dm.pausedDownloads[id] = download
		log.Printf("[MANAGER] Download %s paused", id)
	} else if download.Status == core.StatusQueued {
		download.Status = core.StatusPaused
		dm.pausedDownloads[id] = download
		log.Printf("[MANAGER] Download %s paused (was queued)", id)
	} else {
		return fmt.Errorf("download cannot be paused in current state: %s", download.Status)
	}

	return nil
}

func (dm *DownloadManager) ResumeDownload(id string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	download, exists := dm.downloads[id]
	if !exists {
		return fmt.Errorf("download not found")
	}

	if download.Status != core.StatusPaused {
		return fmt.Errorf("download is not paused")
	}

	// Reset download state and re-queue (yt-dlp will detect partial files and resume)
	download.Status = core.StatusQueued
	download.Error = ""
	// Don't reset CompletedAt as it hasn't completed yet
	// Don't reset progress as yt-dlp will show correct progress when resuming

	// Remove from paused downloads
	delete(dm.pausedDownloads, id)

	// Re-queue the download
	select {
	case dm.queue <- download:
		log.Printf("[MANAGER] Download %s resumed (will continue from partial file if exists)", id)
		return nil
	default:
		download.Status = core.StatusPaused
		dm.pausedDownloads[id] = download
		return fmt.Errorf("download queue is full")
	}
}

func (dm *DownloadManager) RetryDownload(id string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	download, exists := dm.downloads[id]
	if !exists {
		return fmt.Errorf("download not found")
	}

	if download.Status == core.StatusDownloading || download.Status == core.StatusQueued {
		return fmt.Errorf("download is already active")
	}

	// Reset download state
	download.Status = core.StatusQueued
	download.Error = ""
	download.Progress = core.DownloadProgress{}
	download.CompletedAt = nil

	// Re-queue the download
	select {
	case dm.queue <- download:
		return nil
	default:
		return fmt.Errorf("download queue is full")
	}
}

func (dm *DownloadManager) RemoveDownload(id string) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	download, exists := dm.downloads[id]
	if !exists {
		return fmt.Errorf("download not found")
	}

	if download.Status == core.StatusDownloading || download.Status == core.StatusPostProcessing {
		// Cancel the download or post-processing first
		if cancelFunc, exists := dm.cancelFuncs[id]; exists {
			log.Printf("[MANAGER] Cancelling running process for download %s", id)
			cancelFunc()
			delete(dm.cancelFuncs, id)
		}
	}

	// Delete the actual file if it exists (for completed, already exists, or post-processing downloads)
	if download.Status == core.StatusCompleted || 
	   download.Status == core.StatusAlreadyExists || 
	   download.Status == core.StatusPostProcessing {
		if download.OutputPath != "" {
			// Delete the main output file
			if err := os.Remove(download.OutputPath); err != nil {
				log.Printf("[MANAGER] Failed to delete file %s: %v", download.OutputPath, err)
				// Don't return error here - we still want to remove from tracking even if file deletion fails
			} else {
				log.Printf("[MANAGER] Successfully deleted file: %s", download.OutputPath)
			}
		}
	}

	// Clean up temporary files for downloads that were in progress, post-processing, or left in failed/cancelled state
	if download.Status == core.StatusDownloading || 
	   download.Status == core.StatusPostProcessing || 
	   download.Status == core.StatusFailed || 
	   download.Status == core.StatusCancelled {
		dm.cleanupTemporaryFiles(download)
		log.Printf("[MANAGER] Cleaned up temporary files for download %s (status: %s)", download.ID, download.Status)
		
		// Also try to delete any partial output file that might exist for downloading/failed downloads
		if (download.Status == core.StatusDownloading || download.Status == core.StatusFailed) && download.OutputPath != "" {
			if err := os.Remove(download.OutputPath); err != nil {
				// File might not exist yet or might be a temp file, which is fine
				log.Printf("[MANAGER] Note: Could not delete partial file %s: %v", download.OutputPath, err)
			} else {
				log.Printf("[MANAGER] Successfully deleted partial file: %s", download.OutputPath)
			}
		}
	}

	delete(dm.downloads, id)
	delete(dm.pausedDownloads, id)
	delete(dm.cancelFuncs, id)              // Ensure cancel function is removed
	delete(dm.processingUrls, download.URL) // Clean up processing URL
	if ch, exists := dm.progressChannels[id]; exists {
		// Safe channel closing - use recover to handle already closed channels
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, ignore the panic
				}
			}()
			close(ch)
		}()
		delete(dm.progressChannels, id)
	}

	return nil
}

// cleanupTemporaryFiles removes temporary files created during post-processing
func (dm *DownloadManager) cleanupTemporaryFiles(download *core.Download) {
	if download.Filename == "" && download.Title == "" {
		log.Printf("[MANAGER] Cleanup: No filename or title available for download %s", download.ID)
		return
	}

	// Extract the base filename for searching - prioritize OutputPath if available
	baseFilename := download.Filename
	if download.OutputPath != "" {
		// Use the actual output filename as the most accurate base
		outputFile := filepath.Base(download.OutputPath)
		if outputFile != "" {
			baseFilename = outputFile
		}
	}
	if baseFilename == "" && download.Title != "" {
		// Sanitize title to match potential filename patterns
		baseFilename = strings.ToLower(strings.ReplaceAll(download.Title, " ", "_"))
		baseFilename = strings.ReplaceAll(baseFilename, "/", "_")
		baseFilename = strings.ReplaceAll(baseFilename, "\\", "_")
	}

	if baseFilename == "" {
		log.Printf("[MANAGER] Cleanup: No base filename derived for download %s", download.ID)
		return
	}

	// Remove file extension to get base name
	if ext := filepath.Ext(baseFilename); ext != "" {
		baseFilename = strings.TrimSuffix(baseFilename, ext)
	}

	// Look for temporary files in the output directory
	outputDir := dm.outputDir
	if download.OutputPath != "" {
		outputDir = filepath.Dir(download.OutputPath)
	}
	
	log.Printf("[MANAGER] Cleanup: Searching for files matching '%s' in directory: %s", baseFilename, outputDir)

	files, err := os.ReadDir(outputDir)
	if err != nil {
		log.Printf("[MANAGER] Failed to read output directory %s: %v", outputDir, err)
		return
	}

	log.Printf("[MANAGER] Cleanup: Found %d files in directory", len(files))
	
	// First, list all files for debugging
	for _, file := range files {
		if !file.IsDir() {
			log.Printf("[MANAGER] Cleanup: Found file: %s", file.Name())
		}
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		lowerFilename := strings.ToLower(filename)
		lowerBase := strings.ToLower(baseFilename)
		
		log.Printf("[MANAGER] Cleanup: Checking file '%s' against base '%s'", filename, baseFilename)
		
		// Check for files that match the download and might be temporary or yt-dlp format-specific
		if strings.Contains(lowerFilename, lowerBase) {
			log.Printf("[MANAGER] Cleanup: File '%s' contains base name '%s'", filename, baseFilename)
			// Look for common temporary file patterns AND yt-dlp format-specific files
			isTemp := strings.Contains(filename, ".part") ||
				strings.Contains(filename, ".tmp") ||
				strings.Contains(filename, ".temp") ||
				strings.HasSuffix(filename, ".f") || // FFmpeg temp files
				strings.Contains(filename, ".ytdl") ||
				strings.Contains(filename, ".download") ||
				strings.Contains(filename, ".partial") ||
				strings.HasPrefix(filename, "tmp") ||
				strings.HasSuffix(filename, ".downloading")
			
			isYtDlpFormat := dm.isYtDlpFormatFile(filename, baseFilename)
			
			log.Printf("[MANAGER] Cleanup: File '%s' - isTemp: %v, isYtDlpFormat: %v", filename, isTemp, isYtDlpFormat)
			
			if isTemp || isYtDlpFormat {

				fullPath := filepath.Join(outputDir, filename)
				if err := os.Remove(fullPath); err != nil {
					log.Printf("[MANAGER] Failed to delete file %s: %v", fullPath, err)
				} else {
					log.Printf("[MANAGER] Cleaned up file: %s", fullPath)
				}
			} else {
				log.Printf("[MANAGER] Cleanup: File '%s' doesn't match patterns", filename)
			}
		} else {
			log.Printf("[MANAGER] Cleanup: File '%s' does not contain base name '%s'", filename, baseFilename)
			
			// For aggressive cleanup, also check if this looks like a yt-dlp format file
			// even if it doesn't match the base filename exactly
			if dm.isLikelyYtDlpFile(filename) {
				log.Printf("[MANAGER] Cleanup: File '%s' appears to be a yt-dlp format file", filename)
				
				fullPath := filepath.Join(outputDir, filename)
				if err := os.Remove(fullPath); err != nil {
					log.Printf("[MANAGER] Failed to delete likely yt-dlp file %s: %v", fullPath, err)
				} else {
					log.Printf("[MANAGER] Cleaned up likely yt-dlp file: %s", fullPath)
				}
			}
		}
	}
}

// ClearAllQueued marks all downloads in queued status as cancelled and removes them
func (dm *DownloadManager) ClearAllQueued() error {
	func() {
		dm.mutex.Lock()
		defer dm.mutex.Unlock()

		cancelledCount := 0
		deletedCount := 0
		
		for id, download := range dm.downloads {
			if download.Status == core.StatusQueued {
				// First mark as cancelled so workers will skip them when they pick them up from the queue
				download.Status = core.StatusCancelled
				cancelledCount++
				
				// Clean up from processing URLs map to allow re-adding same URL
				delete(dm.processingUrls, download.URL)
				
				// Remove progress channels and cancel functions
				delete(dm.pausedDownloads, id)
				delete(dm.cancelFuncs, id)
				if ch, exists := dm.progressChannels[id]; exists {
					func() {
						defer func() {
							if r := recover(); r != nil {
								// Channel was already closed, ignore
							}
						}()
						close(ch)
					}()
					delete(dm.progressChannels, id)
				}
				
				// Remove from downloads map completely
				delete(dm.downloads, id)
				deletedCount++
			}
		}

		log.Printf("[MANAGER] Cancelled %d queued downloads, deleted %d from tracking", cancelledCount, deletedCount)
	}()
	
	// Save state immediately to persist the cleared queue (outside of mutex)
	if err := dm.SaveState(); err != nil {
		log.Printf("[MANAGER] Warning: Failed to save state after clearing queue: %v", err)
	}
	
	// Start a goroutine to drain any remaining items from the queue
	go dm.drainQueuedItems()
	
	return nil
}

// drainQueuedItems helps drain cancelled items from the queue more quickly
func (dm *DownloadManager) drainQueuedItems() {
	// Give a short timeout for draining to avoid hanging
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	
	drained := 0
	for {
		select {
		case download := <-dm.queue:
			// Check if this download was cancelled/deleted
			dm.mutex.RLock()
			_, exists := dm.downloads[download.ID]
			dm.mutex.RUnlock()
			
			if !exists || download.Status == core.StatusCancelled {
				// This download was cleared, just discard it
				drained++
				log.Printf("[MANAGER] Drained cancelled download from queue: %s", download.ID)
				continue
			} else {
				// This download is still valid, put it back in the queue
				select {
				case dm.queue <- download:
					// Successfully put it back
				default:
					// Queue is full, worker will pick it up later
				}
				// Stop draining since we found a valid item
				log.Printf("[MANAGER] Queue draining stopped after removing %d cancelled items", drained)
				return
			}
		case <-timeout.C:
			// Timeout reached, stop draining
			log.Printf("[MANAGER] Queue draining completed with timeout after removing %d cancelled items", drained)
			return
		default:
			// No more items in queue
			log.Printf("[MANAGER] Queue draining completed after removing %d cancelled items", drained)
			return
		}
	}
}

// isYtDlpFormatFile checks if a filename matches yt-dlp's format-specific file patterns
func (dm *DownloadManager) isYtDlpFormatFile(filename, baseFilename string) bool {
	// yt-dlp often creates files with format patterns like:
	// - video.f137.mp4 (video only)
	// - video.f140.m4a (audio only)  
	// - video.f251.webm (audio in webm)
	// - video.f401.mp4 (video with audio)
	
	log.Printf("[MANAGER] Format check: '%s' vs base '%s'", filename, baseFilename)
	
	lowerFilename := strings.ToLower(filename)
	lowerBase := strings.ToLower(baseFilename)
	
	// Check for .f[NUMBER].extension pattern
	if strings.Contains(lowerFilename, lowerBase) {
		// Look for pattern: .f followed by digits then extension
		parts := strings.Split(filename, ".")
		for i, part := range parts {
			if strings.HasPrefix(part, "f") && len(part) > 1 {
				// Check if the rest are digits
				digits := part[1:]
				if len(digits) > 0 {
					allDigits := true
					for _, r := range digits {
						if r < '0' || r > '9' {
							allDigits = false
							break
						}
					}
					if allDigits && i < len(parts)-1 { // Must have extension after format
						log.Printf("[MANAGER] Format check: Found format pattern f%s in '%s'", digits, filename)
						return true
					}
				}
			}
		}
		
		// Also check for other yt-dlp patterns:
		// - Files with different extensions but same base name
		// - Files with [youtube] or similar prefixes
		// - Files ending with common video/audio extensions that aren't the final output
		baseExt := filepath.Ext(baseFilename)
		filenameExt := filepath.Ext(filename)
		
		if baseExt != "" && filenameExt != "" && baseExt != filenameExt {
			// Different extension, might be separate audio/video stream
			commonExts := []string{".mp4", ".webm", ".m4a", ".mp3", ".mkv", ".flv", ".3gp"}
			for _, ext := range commonExts {
				if filenameExt == ext {
					return true
				}
			}
		}
	}
	
	return false
}

// isLikelyYtDlpFile checks if a filename appears to be a yt-dlp generated file
func (dm *DownloadManager) isLikelyYtDlpFile(filename string) bool {
	lowerFilename := strings.ToLower(filename)
	
	// Check for common yt-dlp file patterns
	patterns := []string{
		".f", // Format files like .f137.mp4
		".part",
		".tmp",
		".temp",
		".ytdl",
		".download",
		".partial",
		".downloading",
	}
	
	// Check if filename contains any of these patterns
	for _, pattern := range patterns {
		if strings.Contains(lowerFilename, pattern) {
			return true
		}
	}
	
	// Check for format-specific patterns (f + digits)
	parts := strings.Split(filename, ".")
	for _, part := range parts {
		if strings.HasPrefix(part, "f") && len(part) > 1 {
			digits := part[1:]
			if len(digits) > 0 {
				allDigits := true
				for _, r := range digits {
					if r < '0' || r > '9' {
						allDigits = false
						break
					}
				}
				if allDigits {
					return true
				}
			}
		}
	}
	
	return false
}

// DeleteAllCompleted removes all completed downloads and their files
func (dm *DownloadManager) DeleteAllCompleted() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	deletedCount := 0
	filesDeleted := 0
	for id, download := range dm.downloads {
		if download.Status == core.StatusCompleted || download.Status == core.StatusAlreadyExists {
			// Delete the actual file if it exists
			if download.OutputPath != "" {
				if err := os.Remove(download.OutputPath); err != nil {
					log.Printf("[MANAGER] Failed to delete file %s: %v", download.OutputPath, err)
				} else {
					filesDeleted++
					log.Printf("[MANAGER] Deleted file: %s", download.OutputPath)
				}
			}

			// Remove from tracking
			delete(dm.downloads, id)
			delete(dm.pausedDownloads, id)
			delete(dm.cancelFuncs, id)
			if ch, exists := dm.progressChannels[id]; exists {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Channel was already closed, ignore
						}
					}()
					close(ch)
				}()
				delete(dm.progressChannels, id)
			}
			deletedCount++
		}
	}

	log.Printf("[MANAGER] Deleted %d completed downloads and %d files", deletedCount, filesDeleted)
	return nil
}

// ClearAllFailed removes all failed downloads from the list
func (dm *DownloadManager) ClearAllFailed() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	clearedCount := 0
	for id, download := range dm.downloads {
		if download.Status == core.StatusFailed {
			// Clean up any temporary files that might have been left behind
			dm.cleanupTemporaryFiles(download)
			
			// Remove from tracking
			delete(dm.downloads, id)
			delete(dm.pausedDownloads, id)
			delete(dm.cancelFuncs, id)
			if ch, exists := dm.progressChannels[id]; exists {
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Channel was already closed, ignore
						}
					}()
					close(ch)
				}()
				delete(dm.progressChannels, id)
			}
			clearedCount++
		}
	}

	log.Printf("[MANAGER] Cleared %d failed downloads and cleaned up temporary files", clearedCount)
	return nil
}

func (dm *DownloadManager) GetProgress(id string) (chan core.DownloadProgress, bool) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	ch, exists := dm.progressChannels[id]
	return ch, exists
}

func (dm *DownloadManager) worker() {
	dm.mutex.Lock()
	dm.activeWorkers++
	workerID := dm.activeWorkers
	dm.mutex.Unlock()

	log.Printf("[MANAGER] Worker %d started", workerID)
	defer func() {
		dm.mutex.Lock()
		dm.activeWorkers--
		dm.mutex.Unlock()
		log.Printf("[MANAGER] Worker %d shutting down", workerID)
	}()

	for {
		select {
		case <-dm.workerCtx.Done():
			return
		case download := <-dm.queue:
			// Check if the download was cancelled or removed while in queue
			dm.mutex.RLock()
			currentDownload, exists := dm.downloads[download.ID]
			dm.mutex.RUnlock()
			
			if !exists {
				log.Printf("[MANAGER] Skipping download %s - no longer exists (was cleared)", download.ID)
				continue
			}
			
			if currentDownload.Status == core.StatusCancelled {
				log.Printf("[MANAGER] Skipping cancelled download %s", download.ID)
				continue
			}
			if currentDownload.Status == core.StatusPaused {
				log.Printf("[MANAGER] Skipping paused download %s", download.ID)
				continue
			}
			
			// Use the current download state, not the queued one
			log.Printf("[MANAGER] Worker processing download %s", currentDownload.ID)
			dm.processDownload(currentDownload)
		}
	}
}

// startWorkers starts the specified number of worker goroutines
func (dm *DownloadManager) startWorkers(count int) {
	for i := 0; i < count; i++ {
		go dm.worker()
	}
}

// UpdateConfig updates the download manager configuration and adjusts workers accordingly
func (dm *DownloadManager) UpdateConfig(newConfig *config.Config) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	oldMaxConcurrent := dm.maxConcurrent
	dm.config = newConfig
	dm.maxConcurrent = newConfig.MaxConcurrentDownloads
	dm.outputDir = newConfig.DownloadPath

	// Create a new downloader with updated paths
	dm.downloader = core.NewDownloader(newConfig.YtDlpPath, newConfig.FfmpegPath, newConfig.EnableHardwareAccel, newConfig.OptimizeForLowPower)

	log.Printf("[MANAGER] Config updated: MaxConcurrent %d -> %d, OutputDir -> %s",
		oldMaxConcurrent, dm.maxConcurrent, dm.outputDir)
	log.Printf("[MANAGER] Downloader updated with new paths: yt-dlp=%s, ffmpeg=%s", 
		newConfig.YtDlpPath, newConfig.FfmpegPath)

	// Adjust workers if needed
	if oldMaxConcurrent != dm.maxConcurrent {
		dm.adjustWorkers(oldMaxConcurrent, dm.maxConcurrent)
	}
}

// adjustWorkers adjusts the number of worker goroutines
func (dm *DownloadManager) adjustWorkers(oldCount, newCount int) {
	if newCount > oldCount {
		// Start additional workers
		additional := newCount - oldCount
		log.Printf("[MANAGER] Starting %d additional workers", additional)
		dm.startWorkers(additional)
	} else if newCount < oldCount {
		// Reduce workers by cancelling and recreating the worker context
		log.Printf("[MANAGER] Reducing workers from %d to %d", oldCount, newCount)

		// Cancel current worker context to stop all workers
		dm.workerCancel()

		// Create new worker context
		dm.workerCtx, dm.workerCancel = context.WithCancel(dm.ctx)

		// Reset active workers count
		dm.activeWorkers = 0

		// Start the new number of workers
		dm.startWorkers(newCount)
	}
}

func (dm *DownloadManager) processDownload(download *core.Download) {
	log.Printf("[MANAGER] Processing download %s", download.ID)

	dm.mutex.Lock()
	// Update status to downloading immediately
	download.Status = core.StatusDownloading
	progressChan := dm.progressChannels[download.ID]
	dm.mutex.Unlock()

	req := core.DownloadRequest{
		URL:       download.URL,
		Type:      download.Type,
		Quality:   download.Quality,
		Format:    download.Format,
		OutputDir: dm.outputDir, // Use the configured output directory
	}

	log.Printf("[MANAGER] Download %s: Creating context and starting download", download.ID)

	// Create a context for this specific download
	ctx, cancel := context.WithCancel(dm.ctx)

	// Store cancel function for potential cancellation
	dm.mutex.Lock()
	dm.cancelFuncs[download.ID] = cancel
	dm.mutex.Unlock()

	defer func() {
		dm.mutex.Lock()
		delete(dm.cancelFuncs, download.ID)
		delete(dm.processingUrls, download.URL)
		dm.mutex.Unlock()
		cancel()
		log.Printf("[MANAGER] Download %s: Context cancelled and cleanup completed", download.ID)
	}()

	// Start progress monitoring goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[MANAGER] Download %s: Progress monitoring goroutine recovered from panic: %v", download.ID, r)
			}
		}()
		for progress := range progressChan {
			dm.mutex.Lock()
			download.Progress = progress
			dm.mutex.Unlock()
		}
		log.Printf("[MANAGER] Download %s: Progress monitoring stopped", download.ID)
	}()

	// Start download
	log.Printf("[MANAGER] Download %s: Calling downloader.Download", download.ID)
	completedDownload, err := dm.downloader.Download(ctx, req, progressChan, dm.UpdateDownloadTitle, dm.UpdateDownloadStatus, download.ID)

	dm.mutex.Lock()
	if ctx.Err() == context.Canceled {
		log.Printf("[MANAGER] Download %s: Cancelled", download.ID)
		download.Status = core.StatusCancelled
	} else if err != nil {
		log.Printf("[MANAGER] Download %s: Failed with error: %v", download.ID, err)
		download.Status = core.StatusFailed
		download.Error = err.Error()
	} else {
		log.Printf("[MANAGER] Download %s: Completed successfully", download.ID)
		download.Status = completedDownload.Status
		download.Title = completedDownload.Title
		download.Filename = completedDownload.Filename
		download.OutputPath = completedDownload.OutputPath
		download.CompletedAt = completedDownload.CompletedAt
	}

	// Close progress channel safely after download completion
	if ch, exists := dm.progressChannels[download.ID]; exists {
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was already closed, ignore the panic
				}
			}()
			close(ch)
		}()
		delete(dm.progressChannels, download.ID)
	}
	dm.mutex.Unlock()
}

func (dm *DownloadManager) UpdateDownloadTitle(id, title string) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if download, exists := dm.downloads[id]; exists {
		if download.Title != title {
			download.Title = title
			log.Printf("[MANAGER] Download %s: Title updated to: %s", id, title)
		}
	}
}

func (dm *DownloadManager) UpdateDownloadStatus(id string, status core.DownloadStatus) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	if download, exists := dm.downloads[id]; exists {
		if download.Status != status {
			download.Status = status
			log.Printf("[MANAGER] Download %s: Status updated to: %s", id, status)
		}
	}
}

func (dm *DownloadManager) cleanupWorker() {
	// Run cleanup every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			log.Printf("[MANAGER] Cleanup worker shutting down")
			return
		case <-ticker.C:
			dm.cleanupExpiredDownloads()
		}
	}
}

func (dm *DownloadManager) cleanupExpiredDownloads() {
	if dm.config.CompletedFileExpiryHours <= 0 {
		return // Auto-expiry disabled
	}

	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	expiryDuration := time.Duration(dm.config.CompletedFileExpiryHours) * time.Hour
	now := time.Now()
	deletedCount := 0

	for id, download := range dm.downloads {
		if download.Status == core.StatusCompleted && download.CompletedAt != nil {
			timeSinceCompletion := now.Sub(*download.CompletedAt)
			if timeSinceCompletion > expiryDuration {
				// Delete the actual file
				if download.OutputPath != "" {
					if err := os.Remove(download.OutputPath); err != nil {
						log.Printf("[MANAGER] Failed to delete expired file %s: %v", download.OutputPath, err)
					} else {
						log.Printf("[MANAGER] Deleted expired file: %s", download.OutputPath)
					}
				}

				// Remove from downloads map
				delete(dm.downloads, id)

				// Clean up progress channel
				if ch, exists := dm.progressChannels[id]; exists {
					func() {
						defer func() {
							if r := recover(); r != nil {
								// Channel was already closed, ignore
							}
						}()
						close(ch)
					}()
					delete(dm.progressChannels, id)
				}

				deletedCount++
			}
		}
	}

	if deletedCount > 0 {
		log.Printf("[MANAGER] Cleanup completed: removed %d expired downloads", deletedCount)
	}
}

// CheckFileExistence checks if a file for the given download request already exists (exported for API use)
func (dm *DownloadManager) CheckFileExistence(req core.DownloadRequest) string {
	return dm.checkFileExists(req)
}

// checkFileExists checks if a file for the given download request already exists
func (dm *DownloadManager) checkFileExists(req core.DownloadRequest) string {
	// Try to get video info to determine potential filename
	info, err := dm.downloader.GetVideoInfo(req.URL)
	if err != nil {
		// If we can't get video info, we can't check for duplicates effectively
		return ""
	}

	// Determine expected file extension
	expectedExt := "." + req.Format

	// List of potential filenames to check
	potentialFilenames := []string{
		core.SanitizeFilename(info.Title) + expectedExt,
		info.Title + expectedExt,
		strings.ReplaceAll(info.Title, " ", "_") + expectedExt,
		strings.ToLower(core.SanitizeFilename(info.Title)) + expectedExt,
	}

	// Check each potential filename
	for _, filename := range potentialFilenames {
		fullPath := filepath.Join(dm.outputDir, filename)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}

	// Also check for similar files in the directory
	files, err := os.ReadDir(dm.outputDir)
	if err != nil {
		return ""
	}

	titleWords := strings.Fields(strings.ToLower(info.Title))
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := strings.ToLower(file.Name())
		// Check if filename has correct extension and contains most title words
		if strings.HasSuffix(filename, expectedExt) {
			matchCount := 0
			for _, word := range titleWords {
				if len(word) > 3 && strings.Contains(filename, word) { // Only check words longer than 3 chars
					matchCount++
				}
			}
			// If more than half the significant words match, consider it a duplicate
			if len(titleWords) > 0 && float64(matchCount)/float64(len(titleWords)) > 0.6 {
				return filepath.Join(dm.outputDir, file.Name())
			}
		}
	}

	return ""
}

// extractTitleFromPath extracts a readable title from a file path
func (dm *DownloadManager) extractTitleFromPath(filePath string) string {
	filename := filepath.Base(filePath)
	// Remove extension
	title := strings.TrimSuffix(filename, filepath.Ext(filename))
	// Replace underscores with spaces
	title = strings.ReplaceAll(title, "_", " ")
	// Basic cleanup
	title = strings.TrimSpace(title)

	if title == "" {
		title = filename
	}

	return title
}

func (dm *DownloadManager) Shutdown() {
	log.Printf("[MANAGER] Shutting down download manager...")

	// Save final state
	if err := dm.SaveState(); err != nil {
		log.Printf("[MANAGER] Failed to save final state: %v", err)
	}

	// Cancel worker context first to stop workers gracefully
	dm.workerCancel()

	// Then cancel main context
	dm.cancel()
	close(dm.queue)

	log.Printf("[MANAGER] Download manager shutdown complete")
}
