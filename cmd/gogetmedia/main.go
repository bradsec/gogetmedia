package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"gogetmedia/internal/api"
	"gogetmedia/internal/config"
	"gogetmedia/internal/core"
	"gogetmedia/internal/manager"
	"gogetmedia/internal/ui"
	"gogetmedia/internal/utils"
)

func ensureDirectories(cfg *config.Config) error {
	// Create download directory
	if err := os.MkdirAll(cfg.DownloadPath, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	// Create assets directory
	assetsDir := filepath.Join("assets", "yt-dlp")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}

	// Create web/public directory for legacy static files
	webPublicDir := filepath.Join("web", "public")
	if err := os.MkdirAll(webPublicDir, 0755); err != nil {
		return fmt.Errorf("failed to create web/public directory: %w", err)
	}

	return nil
}

func main() {
	// Panic recovery for production stability
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ Application panic recovered: %v", r)
			fmt.Printf("❌ Application encountered a critical error and will restart in 5 seconds...\n")
			time.Sleep(5 * time.Second)
			// Try to restart the application
			os.Exit(1)
		}
	}()

	// Command line flags
	var port int
	var configPath string
	flag.IntVar(&port, "port", 0, "Port to run the server on (overrides config file)")
	flag.StringVar(&configPath, "config", "config.json", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override port from command line argument
	if port > 0 {
		cfg.Port = port
	}

	// Override port from environment variable
	if envPort := os.Getenv("GOGETMEDIA_PORT"); envPort != "" {
		if parsedPort, err := strconv.Atoi(envPort); err == nil && parsedPort > 0 {
			cfg.Port = parsedPort
		}
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Set up logging based on config
	utils.SetVerboseLogging(cfg.VerboseLogging)

	// Ensure necessary directories exist
	if err := ensureDirectories(cfg); err != nil {
		log.Fatalf("Failed to create necessary directories: %v", err)
	}

	// Create downloader and manager
	downloader := core.NewDownloader(cfg.YtDlpPath, cfg.FfmpegPath)
	downloadManager := manager.NewDownloadManager(downloader, cfg.MaxConcurrentDownloads, cfg.DownloadPath, cfg)

	// Create updater
	updater := core.NewYtDlpUpdater(cfg.YtDlpPath, filepath.Join("assets", "yt-dlp"))

	// Check for yt-dlp and auto-download if needed
	fmt.Printf("Checking yt-dlp availability...\n")
	if _, err := os.Stat(cfg.YtDlpPath); os.IsNotExist(err) {
		fmt.Printf("yt-dlp not found, downloading...\n")
		if err := updater.Update(); err != nil {
			fmt.Printf("Warning: Failed to download yt-dlp: %v\n", err)
		} else {
			fmt.Printf("✓ yt-dlp downloaded successfully\n")
		}
	} else {
		// Check for updates
		updateInfo, err := updater.CheckForUpdates()
		if err != nil {
			utils.LogInfo("Failed to check for yt-dlp updates: %v", err)
		} else {
			if updateInfo.UpdateAvailable {
				fmt.Printf("yt-dlp update available: %s → %s\n", updateInfo.CurrentVersion, updateInfo.LatestVersion)
				fmt.Printf("Downloading latest yt-dlp version...\n")
				if err := updater.Update(); err != nil {
					fmt.Printf("Warning: Failed to update yt-dlp: %v\n", err)
				} else {
					fmt.Printf("✓ yt-dlp updated to version %s\n", updateInfo.LatestVersion)
				}
			} else {
				fmt.Printf("✓ yt-dlp is up to date (%s)\n", updateInfo.CurrentVersion)
			}
		}
	}

	// Check for ffmpeg availability - REQUIRED
	fmt.Printf("Checking ffmpeg availability...\n")
	if !core.CheckFfmpegAvailable(cfg.FfmpegPath) {
		fmt.Printf("❌ ffmpeg not found at %s or in system PATH\n", cfg.FfmpegPath)
		fmt.Printf("\nffmpeg is required for this application to function properly.\n")
		fmt.Printf("Please either:\n")
		fmt.Printf("  1. Install ffmpeg system-wide and ensure it's in your PATH\n")
		fmt.Printf("  2. Download ffmpeg and set the correct path in the application settings\n")
		fmt.Printf("\nThe application will now start in settings-only mode.\n")
		fmt.Printf("You must configure a valid ffmpeg path before downloads will work.\n\n")
	} else {
		// Get ffmpeg version info
		versions := core.GetVersionInfo(cfg.YtDlpPath, cfg.FfmpegPath)
		if versions.FfmpegVersion != "" {
			fmt.Printf("✓ ffmpeg is available (%s)\n", versions.FfmpegVersion)
		} else {
			fmt.Printf("✓ ffmpeg is available\n")
		}
	}

	// Create handlers
	apiHandler := api.NewHandler(cfg, configPath, downloadManager, updater)
	uiHandler := ui.NewTemplateHandler(cfg)

	// Setup routes
	router := api.SetupRoutes(apiHandler, ui.Assets)

	// Add UI route
	router.HandleFunc("/", uiHandler.ServeIndex).Methods("GET")

	// Ensure web/public directory exists
	if err := os.MkdirAll(filepath.Join("web", "public"), 0755); err != nil {
		log.Printf("Warning: Failed to create web/public directory: %v", err)
	}

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("Starting GoGetMedia server...\n")
	fmt.Printf("Port: %d\n", cfg.Port)
	fmt.Printf("Download path: %s\n", cfg.DownloadPath)
	fmt.Printf("yt-dlp path: %s\n", cfg.YtDlpPath)
	fmt.Printf("ffmpeg path: %s\n", cfg.FfmpegPath)
	fmt.Printf("Max concurrent downloads: %d\n", cfg.MaxConcurrentDownloads)
	fmt.Printf("\nInitializing components and starting web server...\n")

	// Create a custom server to show when it's ready
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Create listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Printf("\n❌ Failed to start server on port %d: %v\n", cfg.Port, err)
		fmt.Printf("\nTo change the port, you can:\n")
		fmt.Printf("1. Edit config.json and change the \"port\" value\n")
		fmt.Printf("2. Use command line: ./gogetmedia -port 3000\n")
		fmt.Printf("3. Use environment variable: GOGETMEDIA_PORT=3000 ./gogetmedia\n")
		fmt.Printf("4. Use -h flag to see all available options\n")
		os.Exit(1)
	}

	fmt.Printf("✓ Server is ready and listening on http://localhost%s\n", addr)
	fmt.Printf("✓ Web UI is now available - you can access it in your browser\n")
	fmt.Printf("\nTo change port: Edit config.json, use -port flag, or GOGETMEDIA_PORT env var\n")
	fmt.Printf("Press Ctrl+C to stop the server\n")
	fmt.Printf("=====================================\n")

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	// Wait for shutdown signal or server error
	for {
		select {
		case sig := <-sigChan:
			fmt.Printf("\n\nReceived %s signal, shutting down gracefully...\n", sig)

			// Create shutdown context with timeout
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			// Shutdown download manager
			fmt.Printf("Stopping download manager...\n")
			downloadManager.Shutdown()

			// Shutdown HTTP server
			fmt.Printf("Stopping HTTP server...\n")
			if err := server.Shutdown(shutdownCtx); err != nil {
				fmt.Printf("Error during server shutdown: %v\n", err)
			}

			fmt.Printf("✓ Server shutdown complete\n")
			return

		case err := <-serverErrChan:
			fmt.Printf("\n❌ Server error: %v\n", err)

			// Attempt graceful cleanup
			downloadManager.Shutdown()

			// Give some time for cleanup
			time.Sleep(2 * time.Second)

			fmt.Printf("Attempting to restart server in 5 seconds...\n")
			time.Sleep(5 * time.Second)

			// Restart the server
			fmt.Printf("Restarting server...\n")
			if newListener, err := net.Listen("tcp", addr); err == nil {
				listener = newListener
				go func() {
					if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
						serverErrChan <- err
					}
				}()
				fmt.Printf("✓ Server restarted successfully\n")
			} else {
				fmt.Printf("❌ Failed to restart server: %v\n", err)
				os.Exit(1)
			}
		}
	}
}
