package api

import (
	"github.com/gorilla/mux"
	"io/fs"
	"net/http"
	"path/filepath"
)

func SetupRoutes(handler *Handler, assetsFS fs.FS) *mux.Router {
	router := mux.NewRouter()

	// CORS middleware
	router.Use(corsMiddleware)

	// API routes
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/config", handler.GetConfig).Methods("GET")
	api.HandleFunc("/config", handler.UpdateConfig).Methods("POST")
	api.HandleFunc("/downloads", handler.GetDownloads).Methods("GET")
	api.HandleFunc("/downloads", handler.StartDownload).Methods("POST")
	api.HandleFunc("/downloads/playlist", handler.StartPlaylistDownload).Methods("POST")
	api.HandleFunc("/downloads/first-video", handler.StartFirstVideoDownload).Methods("POST")
	api.HandleFunc("/downloads/{id}", handler.DeleteDownload).Methods("DELETE")
	api.HandleFunc("/downloads/{id}/cancel", handler.CancelDownload).Methods("POST")
	api.HandleFunc("/downloads/{id}/pause", handler.PauseDownload).Methods("POST")
	api.HandleFunc("/downloads/{id}/resume", handler.ResumeDownload).Methods("POST")
	api.HandleFunc("/downloads/{id}/retry", handler.RetryDownload).Methods("POST")
	api.HandleFunc("/downloads/{id}/download", handler.DownloadFile).Methods("GET")
	api.HandleFunc("/downloads/clear-queued", handler.ClearAllQueued).Methods("POST")
	api.HandleFunc("/downloads/delete-completed", handler.DeleteAllCompleted).Methods("POST")
	api.HandleFunc("/downloads/clear-failed", handler.ClearAllFailed).Methods("POST")
	api.HandleFunc("/validate", handler.ValidateURL).Methods("POST")
	api.HandleFunc("/yt-dlp/version", handler.GetUpdateInfo).Methods("GET")
	api.HandleFunc("/yt-dlp/update", handler.UpdateYtDlp).Methods("POST")
	api.HandleFunc("/versions", handler.GetVersions).Methods("GET")

	// Static files (legacy)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join("web", "public")))))

	// Assets (embedded)
	assetsSubFS, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		// Fallback to full path if Sub fails
		assetsHandler := http.FileServer(http.FS(assetsFS))
		router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", assetsHandler))
	} else {
		assetsHandler := http.FileServer(http.FS(assetsSubFS))
		router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", assetsHandler))
	}

	return router
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
