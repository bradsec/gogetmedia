package api

import (
	"bytes"
	"encoding/json"
	"gogetmedia/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewHandler(cfg, "test_config.json", nil, nil)

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	handler.GetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response config.Config
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Port != cfg.Port {
		t.Errorf("Expected port %d, got %d", cfg.Port, response.Port)
	}
}

func TestStartDownload(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewHandler(cfg, "test_config.json", nil, nil)

	requestBody := map[string]string{
		"url":     "https://example.com/video",
		"type":    "video",
		"quality": "720p",
		"format":  "mp4",
	}

	jsonBody, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/api/downloads", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.StartDownload(w, req)

	// Since we're passing nil for download manager, we expect an error
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestStartDownloadInvalidJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewHandler(cfg, "test_config.json", nil, nil)

	req := httptest.NewRequest("POST", "/api/downloads", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.StartDownload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestStartDownloadMissingURL(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewHandler(cfg, "test_config.json", nil, nil)

	requestBody := map[string]string{
		"type":    "video",
		"quality": "720p",
		"format":  "mp4",
	}

	jsonBody, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/api/downloads", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.StartDownload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetDownloads(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewHandler(cfg, "test_config.json", nil, nil)

	req := httptest.NewRequest("GET", "/api/downloads", nil)
	w := httptest.NewRecorder()

	handler.GetDownloads(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}

func TestDeleteDownload(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewHandler(cfg, "test_config.json", nil, nil)

	req := httptest.NewRequest("DELETE", "/api/downloads/123", nil)
	w := httptest.NewRecorder()

	handler.DeleteDownload(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}
}
