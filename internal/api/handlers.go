package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"gogetmedia/internal/config"
	"gogetmedia/internal/core"
	"gogetmedia/internal/manager"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
)

type Handler struct {
	config          *config.Config
	configPath      string
	downloadManager *manager.DownloadManager
	updater         *core.YtDlpUpdater
}

func NewHandler(cfg *config.Config, configPath string, dm *manager.DownloadManager, updater *core.YtDlpUpdater) *Handler {
	return &Handler{
		config:          cfg,
		configPath:      configPath,
		downloadManager: dm,
		updater:         updater,
	}
}

func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.config)
}

func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig config.Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := newConfig.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := newConfig.Save(h.configPath); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	h.config = &newConfig

	// Update the download manager configuration
	if h.downloadManager != nil {
		h.downloadManager.UpdateConfig(&newConfig)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.config)
}

func (h *Handler) GetDownloads(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	downloads := h.downloadManager.GetAllDownloads()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(downloads)
}

func (h *Handler) StartDownload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		URL     string `json:"url"`
		Type    string `json:"type"`    // "video" or "audio"
		Quality string `json:"quality"` // "best", "worst", "720p", etc.
		Format  string `json:"format"`  // "mp4", "mp3", etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("[API] StartDownload: Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[API] StartDownload request: URL=%s, Type=%s, Quality=%s, Format=%s", request.URL, request.Type, request.Quality, request.Format)

	if request.URL == "" {
		log.Printf("[API] StartDownload: URL is required")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Check if ffmpeg is required and available
	var downloadType core.DownloadType
	if request.Type == "audio" {
		downloadType = core.AudioDownload
	} else {
		downloadType = core.VideoDownload
	}

	if core.RequiresFfmpeg(downloadType, request.Format) && !core.CheckFfmpegAvailable(h.config.FfmpegPath) {
		log.Printf("[API] StartDownload: ffmpeg required but not available for %s/%s", request.Type, request.Format)
		http.Error(w, "ffmpeg is required for this download format but is not available. Please configure a valid ffmpeg path in settings.", http.StatusBadRequest)
		return
	}


	// Create download request
	req := core.DownloadRequest{
		URL:       request.URL,
		Type:      downloadType,
		Quality:   request.Quality,
		Format:    request.Format,
		OutputDir: h.config.DownloadPath,
	}

	// Add to download manager
	if h.downloadManager == nil {
		log.Printf("[API] StartDownload: Download manager not initialized")
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] StartDownload: Adding download to manager")
	download, err := h.downloadManager.AddDownload(req)
	if err != nil {
		log.Printf("[API] StartDownload: Failed to add download: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[API] StartDownload: Download added successfully with ID: %s", download.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(download)
}

func (h *Handler) StartPlaylistDownload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		URL     string `json:"url"`
		Type    string `json:"type"`    // "video" or "audio"
		Quality string `json:"quality"` // "best", "worst", "720p", etc.
		Format  string `json:"format"`  // "mp4", "mp3", etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("[API] StartPlaylistDownload: Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[API] StartPlaylistDownload request: URL=%s, Type=%s, Quality=%s, Format=%s", request.URL, request.Type, request.Quality, request.Format)

	if request.URL == "" {
		log.Printf("[API] StartPlaylistDownload: URL is required")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Convert to download type
	var downloadType core.DownloadType
	if request.Type == "audio" {
		downloadType = core.AudioDownload
	} else {
		downloadType = core.VideoDownload
	}

	// Check if ffmpeg is required and available
	if core.RequiresFfmpeg(downloadType, request.Format) && !core.CheckFfmpegAvailable(h.config.FfmpegPath) {
		log.Printf("[API] StartPlaylistDownload: ffmpeg required but not available for %s/%s", request.Type, request.Format)
		http.Error(w, "ffmpeg is required for this download format but is not available. Please configure a valid ffmpeg path in settings.", http.StatusBadRequest)
		return
	}

	// Create download request
	req := core.DownloadRequest{
		URL:       request.URL,
		Type:      downloadType,
		Quality:   request.Quality,
		Format:    request.Format,
		OutputDir: h.config.DownloadPath,
	}

	// Add playlist to download manager
	if h.downloadManager == nil {
		log.Printf("[API] StartPlaylistDownload: Download manager not initialized")
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] StartPlaylistDownload: Adding playlist to manager")
	download, err := h.downloadManager.AddPlaylistDownload(req)
	if err != nil {
		log.Printf("[API] StartPlaylistDownload: Failed to add playlist: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[API] StartPlaylistDownload: Playlist added successfully with first download ID: %s", download.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "Playlist download started",
		"first_download": download,
	})
}

func (h *Handler) StartFirstVideoDownload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		URL     string `json:"url"`
		Type    string `json:"type"`    // "video" or "audio"
		Quality string `json:"quality"` // "best", "worst", "720p", etc.
		Format  string `json:"format"`  // "mp4", "mp3", etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("[API] StartFirstVideoDownload: Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[API] StartFirstVideoDownload request: URL=%s, Type=%s, Quality=%s, Format=%s", request.URL, request.Type, request.Quality, request.Format)

	if request.URL == "" {
		log.Printf("[API] StartFirstVideoDownload: URL is required")
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Create downloader to get playlist items
	downloader := core.NewDownloader(h.config.YtDlpPath, h.config.FfmpegPath)
	playlistItems, err := downloader.GetPlaylistItems(request.URL)
	if err != nil {
		log.Printf("[API] StartFirstVideoDownload: Failed to get playlist items: %v", err)
		http.Error(w, "Failed to get playlist items: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(playlistItems) == 0 {
		log.Printf("[API] StartFirstVideoDownload: No items found in playlist")
		http.Error(w, "No items found in playlist", http.StatusBadRequest)
		return
	}

	// Get the first video URL
	firstVideoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", playlistItems[0].ID)

	// Convert to download type
	var downloadType core.DownloadType
	if request.Type == "audio" {
		downloadType = core.AudioDownload
	} else {
		downloadType = core.VideoDownload
	}

	// Create download request for first video only
	req := core.DownloadRequest{
		URL:       firstVideoURL,
		Type:      downloadType,
		Quality:   request.Quality,
		Format:    request.Format,
		OutputDir: h.config.DownloadPath,
	}

	// Add to download manager
	if h.downloadManager == nil {
		log.Printf("[API] StartFirstVideoDownload: Download manager not initialized")
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	log.Printf("[API] StartFirstVideoDownload: Adding first video to manager")
	download, err := h.downloadManager.AddDownload(req)
	if err != nil {
		log.Printf("[API] StartFirstVideoDownload: Failed to add download: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[API] StartFirstVideoDownload: First video download added successfully with ID: %s", download.ID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(download)
}

func (h *Handler) ValidateURL(w http.ResponseWriter, r *http.Request) {
	var request struct {
		URL     string `json:"url"`
		Type    string `json:"type"`    // "video" or "audio"
		Quality string `json:"quality"` // "best", "worst", "720p", etc.
		Format  string `json:"format"`  // "mp4", "mp3", etc.
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if request.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Create a temporary downloader to validate the URL
	downloader := core.NewDownloader(h.config.YtDlpPath, h.config.FfmpegPath)

	// Check if this is a playlist URL
	isPlaylist := downloader.IsPlaylistURL(request.URL)

	response := map[string]interface{}{
		"valid":       false,
		"is_playlist": isPlaylist,
	}

	if isPlaylist {
		// Get playlist information
		playlistItems, err := downloader.GetPlaylistItems(request.URL)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"valid":       false,
				"error":       err.Error(),
				"is_playlist": true,
			})
			return
		}

		// Get info for the first video in the playlist
		var firstVideoInfo *core.VideoInfo
		if len(playlistItems) > 0 {
			firstVideoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", playlistItems[0].ID)
			firstVideoInfo, _ = downloader.GetVideoInfo(firstVideoURL)
		}

		response["valid"] = true
		response["playlist_count"] = len(playlistItems)
		response["playlist_title"] = "Playlist"
		if len(playlistItems) > 0 {
			response["first_video_title"] = playlistItems[0].Title
			if firstVideoInfo != nil {
				response["first_video_title"] = firstVideoInfo.Title
			}
		}

		// Check if first video file already exists
		if len(playlistItems) > 0 {
			var downloadType core.DownloadType
			if request.Type == "audio" {
				downloadType = core.AudioDownload
			} else {
				downloadType = core.VideoDownload
			}

			firstVideoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", playlistItems[0].ID)
			downloadReq := core.DownloadRequest{
				URL:       firstVideoURL,
				Type:      downloadType,
				Quality:   request.Quality,
				Format:    request.Format,
				OutputDir: h.config.DownloadPath,
			}

			existingFile := h.downloadManager.CheckFileExistence(downloadReq)
			response["first_video_exists"] = existingFile != ""
			if existingFile != "" {
				response["existing_file"] = existingFile
				response["existing_filename"] = filepath.Base(existingFile)
			}
		}
	} else {
		// Single video validation
		info, err := downloader.GetVideoInfo(request.URL)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"valid":       false,
				"error":       err.Error(),
				"is_playlist": false,
			})
			return
		}

		// Check if file already exists
		var downloadType core.DownloadType
		if request.Type == "audio" {
			downloadType = core.AudioDownload
		} else {
			downloadType = core.VideoDownload
		}

		downloadReq := core.DownloadRequest{
			URL:       request.URL,
			Type:      downloadType,
			Quality:   request.Quality,
			Format:    request.Format,
			OutputDir: h.config.DownloadPath,
		}

		// Check for existing file using the download manager's logic
		existingFile := h.downloadManager.CheckFileExistence(downloadReq)

		response["valid"] = true
		response["title"] = info.Title
		response["filename"] = info.Filename
		response["file_exists"] = existingFile != ""

		if existingFile != "" {
			response["existing_file"] = existingFile
			response["existing_filename"] = filepath.Base(existingFile)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) DeleteDownload(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "Download ID is required", http.StatusBadRequest)
		return
	}

	if err := h.downloadManager.RemoveDownload(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CancelDownload(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "Download ID is required", http.StatusBadRequest)
		return
	}

	if err := h.downloadManager.CancelDownload(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

func (h *Handler) PauseDownload(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "Download ID is required", http.StatusBadRequest)
		return
	}

	if err := h.downloadManager.PauseDownload(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

func (h *Handler) ResumeDownload(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "Download ID is required", http.StatusBadRequest)
		return
	}

	if err := h.downloadManager.ResumeDownload(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

func (h *Handler) RetryDownload(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "Download ID is required", http.StatusBadRequest)
		return
	}

	if err := h.downloadManager.RetryDownload(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "retried"})
}

func (h *Handler) ClearAllQueued(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	if err := h.downloadManager.ClearAllQueued(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared", "message": "All queued downloads cleared"})
}

func (h *Handler) DeleteAllCompleted(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	if err := h.downloadManager.DeleteAllCompleted(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "message": "All completed downloads and files deleted"})
}

func (h *Handler) ClearAllFailed(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	if err := h.downloadManager.ClearAllFailed(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared", "message": "All failed downloads cleared"})
}

func (h *Handler) GetUpdateInfo(w http.ResponseWriter, r *http.Request) {
	if h.updater == nil {
		http.Error(w, "Updater not initialized", http.StatusInternalServerError)
		return
	}

	updateInfo, err := h.updater.CheckForUpdates()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to check for updates: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updateInfo)
}

func (h *Handler) UpdateYtDlp(w http.ResponseWriter, r *http.Request) {
	if h.updater == nil {
		http.Error(w, "Updater not initialized", http.StatusInternalServerError)
		return
	}

	if err := h.updater.Update(); err != nil {
		http.Error(w, fmt.Sprintf("Update failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	response := map[string]string{
		"status":  "success",
		"message": "yt-dlp updated successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) GetVersions(w http.ResponseWriter, r *http.Request) {
	versions := core.GetVersionInfo(h.config.YtDlpPath, h.config.FfmpegPath)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}

func (h *Handler) CheckFfmpeg(w http.ResponseWriter, r *http.Request) {
	configuredPath := h.config.FfmpegPath
	var version string
	var actualPath string
	var available bool
	
	// Check the configured path first
	if core.IsCommandAvailable(configuredPath) {
		available = true
		actualPath = configuredPath
	} else if core.IsCommandAvailable("ffmpeg") {
		// Found in system PATH
		available = true
		actualPath = "ffmpeg (system PATH)"
	} else {
		available = false
		actualPath = configuredPath
	}
	
	if available {
		versions := core.GetVersionInfo(h.config.YtDlpPath, h.config.FfmpegPath)
		version = versions.FfmpegVersion
	}
	
	response := map[string]interface{}{
		"available":       available,
		"version":         version,
		"configured_path": configuredPath,
		"actual_path":     actualPath,
		"path":            actualPath, // For backward compatibility
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	if h.downloadManager == nil {
		http.Error(w, "Download manager not initialized", http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		http.Error(w, "Download ID is required", http.StatusBadRequest)
		return
	}

	download, exists := h.downloadManager.GetDownload(id)
	if !exists {
		http.Error(w, "Download not found", http.StatusNotFound)
		return
	}

	if download.Status != core.StatusCompleted {
		http.Error(w, fmt.Sprintf("Download not completed (status: %s)", download.Status), http.StatusBadRequest)
		return
	}

	// Debug: log the file path being checked
	fmt.Printf("Checking file path: %s\n", download.OutputPath)

	// Check if file exists
	if _, err := os.Stat(download.OutputPath); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("File not found at path: %s", download.OutputPath), http.StatusNotFound)
		return
	}

	// Open file
	file, err := os.Open(download.OutputPath)
	if err != nil {
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to get file info", http.StatusInternalServerError)
		return
	}

	// Set headers for download
	// Use the original title if available, otherwise use the actual filename
	var downloadFilename string
	if download.Title != "" && download.Title != download.URL {
		// Use the original title with the correct extension, properly sanitized
		ext := filepath.Ext(download.OutputPath)
		// Use the same sanitization function as the download process
		cleanTitle := core.SanitizeFilename(download.Title)
		downloadFilename = cleanTitle + ext
		log.Printf("[API] Using sanitized title as filename: %s -> %s", download.Title, downloadFilename)
	} else {
		// Fall back to the actual filename
		downloadFilename = filepath.Base(download.OutputPath)
		log.Printf("[API] Using base filename: %s", downloadFilename)
	}

	log.Printf("[API] Final download filename: %s (Title: '%s', OutputPath: '%s')", downloadFilename, download.Title, download.OutputPath)

	// URL encode the filename to handle special characters
	encodedFilename := url.QueryEscape(downloadFilename)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"; filename*=UTF-8''%s", downloadFilename, encodedFilename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Stream file to response
	io.Copy(w, file)
}
