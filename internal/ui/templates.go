package ui

import (
	"embed"
	"gogetmedia/internal/config"
	"net/http"
)

//go:embed assets
var Assets embed.FS

type TemplateHandler struct {
	config *config.Config
}

func NewTemplateHandler(cfg *config.Config) *TemplateHandler {
	return &TemplateHandler{config: cfg}
}

func (th *TemplateHandler) ServeIndex(w http.ResponseWriter, r *http.Request) {
	// Using Tailwind CDN for better styling - this is what was working before
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GoGetMedia - A WebUI for yt-dlp</title>
    <link rel="stylesheet" href="/assets/css/tailwind.min.css">
    <script src="https://unpkg.com/vue@3/dist/vue.global.js"></script>
    <style>
        .gradient-bg {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .card-shadow {
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
        }
        .btn-primary {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            transition: all 0.3s ease;
        }
        .btn-primary:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 20px rgba(0, 0, 0, 0.2);
        }
        .section-header {
            transition: all 0.3s ease;
            cursor: pointer;
        }
        .section-header:hover {
            transform: translateY(-1px);
        }
        .accordion-content {
            transition: max-height 0.3s ease, opacity 0.3s ease;
            overflow: hidden;
        }
        .accordion-collapsed {
            max-height: 0;
            opacity: 0;
        }
        .accordion-expanded {
            max-height: 2000px;
            opacity: 1;
        }
        .download-list-container {
            min-height: 60px; /* Prevent jumping when empty */
            transition: all 0.2s ease;
        }
        .download-list-scrollable {
            max-height: 400px; /* Slightly taller than 96 for better UX */
            overflow-y: auto;
            overflow-x: hidden;
            scroll-behavior: smooth;
            scrollbar-width: thin;
            scrollbar-color: rgb(148 163 184) transparent;
        }
        .download-list-scrollable::-webkit-scrollbar {
            width: 8px;
        }
        .download-list-scrollable::-webkit-scrollbar-track {
            background: transparent;
            border-radius: 4px;
        }
        .download-list-scrollable::-webkit-scrollbar-thumb {
            background-color: rgb(148 163 184);
            border-radius: 4px;
            border: 1px solid transparent;
            background-clip: content-box;
        }
        .download-list-scrollable::-webkit-scrollbar-thumb:hover {
            background-color: rgb(100 116 139);
        }
        .download-item {
            transition: all 0.2s ease;
        }
        .download-item:hover {
            transform: translateY(-1px);
        }
        .progress-bar {
            height: 10px;
            background: #e5e7eb;
            border-radius: 6px;
            overflow: hidden;
            margin: 10px 0;
            width: 100%;
            min-width: 0;
        }
        .dark .progress-bar {
            background: #374151;
        }
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #3b82f6, #8b5cf6);
            transition: width 0.3s ease;
            border-radius: 6px;
            min-width: 2px;
        }
        @media (max-width: 640px) {
            .progress-bar {
                height: 12px;
                margin: 12px 0;
            }
            .download-item {
                margin: 0;
                border-radius: 12px;
            }
            .download-item h3 {
                font-size: 14px;
                line-height: 1.3;
            }
            .status-badge {
                font-size: 10px;
                padding: 4px 8px;
            }
            .section-header {
                padding: 16px !important;
            }
            .section-header h2 {
                font-size: 18px !important;
            }
        }
        .overflow-y-auto {
            scroll-behavior: smooth;
            scrollbar-width: thin;
            scrollbar-color: rgb(148 163 184) transparent;
        }
        .overflow-y-auto::-webkit-scrollbar {
            width: 8px;
        }
        .overflow-y-auto::-webkit-scrollbar-track {
            background: transparent;
        }
        .overflow-y-auto::-webkit-scrollbar-thumb {
            background-color: rgb(148 163 184);
            border-radius: 4px;
            border: 2px solid transparent;
            background-clip: content-box;
        }
        .overflow-y-auto::-webkit-scrollbar-thumb:hover {
            background-color: rgb(100 116 139);
        }
        .processing-indicator {
            display: flex;
            align-items: center;
            gap: 8px;
            padding: 8px 0;
            flex-wrap: wrap;
        }
        @media (max-width: 640px) {
            .processing-indicator {
                flex-direction: column;
                align-items: flex-start;
                gap: 6px;
            }
        }
        .processing-dots {
            display: flex;
            gap: 4px;
        }
        .processing-dot {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: linear-gradient(90deg, #8b5cf6, #a855f7);
            animation: processingPulse 1.5s ease-in-out infinite;
        }
        .processing-dot:nth-child(2) {
            animation-delay: 0.2s;
        }
        .processing-dot:nth-child(3) {
            animation-delay: 0.4s;
        }
        @keyframes processingPulse {
            0%, 80%, 100% {
                transform: scale(0.8);
                opacity: 0.5;
            }
            40% {
                transform: scale(1.2);
                opacity: 1;
            }
        }
        .processing-spinner {
            width: 16px;
            height: 16px;
            border: 2px solid #e5e7eb;
            border-top: 2px solid #8b5cf6;
            border-radius: 50%;
            animation: spin 1s linear infinite;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        .status-badge {
            display: inline-flex;
            align-items: center;
            padding: 4px 12px;
            border-radius: 6px;
            font-size: 12px;
            font-weight: 500;
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }
        .status-queued {
            background: #f59e0b;
            color: white;
        }
        .status-downloading {
            background: #3b82f6;
            color: white;
        }
        .status-post-processing {
            background: #8b5cf6;
            color: white;
        }
        .status-completed {
            background: #10b981;
            color: white;
        }
        .status-failed {
            background: #ef4444;
            color: white;
        }
    </style>
</head>
<body class="bg-gradient-to-br from-slate-50 via-blue-50 to-indigo-50 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 min-h-screen transition-all duration-500">
    <div id="app" class="container mx-auto px-4 py-8">
        <!-- Header -->
        <div class="relative overflow-hidden rounded-3xl bg-white dark:bg-slate-800 shadow-2xl mb-8 card-shadow">
            <div class="gradient-bg absolute inset-0 opacity-10"></div>
            <div class="relative px-8 py-12">
                <div class="flex flex-col lg:flex-row justify-between items-start lg:items-center space-y-6 lg:space-y-0">
                    <div class="flex items-center space-x-4">
                        <div class="p-3 bg-gradient-to-r from-blue-500 to-purple-600 rounded-2xl shadow-lg">
                            <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10"/>
                            </svg>
                        </div>
                        <div>
                            <h1 class="text-4xl font-bold bg-gradient-to-r from-blue-600 to-purple-600 bg-clip-text text-transparent">
                                GoGetMedia
                            </h1>
                            <p class="text-slate-600 dark:text-slate-400 text-lg">A WebUI for yt-dlp</p>
                        </div>
                    </div>
                    <div class="flex flex-col sm:flex-row items-start sm:items-center space-y-4 sm:space-y-0 sm:space-x-6">
                        <div class="flex items-center space-x-3">
                            <div class="flex items-center space-x-2">
                                <div class="w-2 h-2 rounded-full" :class="isConnected ? 'bg-green-500 animate-pulse' : 'bg-red-500'"></div>
                                <span class="text-sm font-medium" :class="isConnected ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">
                                    {{ isConnected ? 'Connected' : 'Disconnected' }}
                                </span>
                            </div>
                            <!-- Update notification -->
                            <div v-if="updateInfo && updateInfo.update_available" class="flex items-center space-x-2 bg-amber-100 dark:bg-amber-900 border border-amber-300 dark:border-amber-700 rounded-lg px-3 py-1">
                                <svg class="w-4 h-4 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"/>
                                </svg>
                                <span class="text-xs font-medium text-amber-800 dark:text-amber-200">yt-dlp update available</span>
                            </div>
                        </div>
                        <div class="flex flex-wrap gap-3">
                            <button @click="showSettings = true" class="group flex items-center px-4 py-2 bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300 rounded-xl hover:bg-slate-200 dark:hover:bg-slate-600 transition-all duration-200 shadow-md hover:shadow-lg">
                                <svg class="w-5 h-5 mr-2 group-hover:rotate-45 transition-transform duration-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
                                </svg>
                                <span class="text-sm font-medium">Settings</span>
                            </button>
                            <button @click="toggleDarkMode" class="group flex items-center px-4 py-2 bg-slate-100 dark:bg-slate-700 text-slate-700 dark:text-slate-300 rounded-xl hover:bg-slate-200 dark:hover:bg-slate-600 transition-all duration-200 shadow-md hover:shadow-lg">
                                <svg v-if="isDarkMode" class="w-5 h-5 mr-2 group-hover:rotate-180 transition-transform duration-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/>
                                </svg>
                                <svg v-else class="w-5 h-5 mr-2 group-hover:rotate-180 transition-transform duration-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"/>
                                </svg>
                                <span class="text-sm font-medium">{{ isDarkMode ? 'Light' : 'Dark' }}</span>
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Add New Download Form -->
        <div class="max-w-4xl mx-auto mb-8 bg-white dark:bg-slate-800 rounded-2xl shadow-2xl card-shadow">
            <div class="px-8 py-6 border-b border-slate-200 dark:border-slate-700">
                <div class="flex items-center space-x-4">
                    <div class="p-3 bg-gradient-to-r from-blue-500 to-purple-600 rounded-2xl shadow-lg">
                        <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6"/>
                        </svg>
                    </div>
                    <h2 class="text-lg sm:text-2xl font-bold text-slate-800 dark:text-white">Add New Download</h2>
                </div>
            </div>
            
            <div class="p-8">
                <form @submit.prevent="handleFormSubmit" class="space-y-6">
                    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        <div class="lg:col-span-2">
                            <label for="url" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Video/Audio URL</label>
                            <input 
                                type="url" 
                                id="url"
                                v-model="newDownload.url" 
                                @input="validateUrl"
                                placeholder="https://www.youtube.com/watch?v=..." 
                                class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors"
                                required
                            >
                        </div>
                        
                        <!-- Playlist Detection Section -->
                        <div v-if="playlistInfo && playlistInfo.is_playlist" class="lg:col-span-2 bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900 dark:to-indigo-900 border border-blue-200 dark:border-blue-700 rounded-xl p-4">
                            <div class="flex items-center mb-3">
                                <svg class="w-6 h-6 text-blue-500 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/>
                                </svg>
                                <h3 class="text-lg font-semibold text-blue-800 dark:text-blue-200">Playlist Detected</h3>
                            </div>
                            <p class="text-blue-700 dark:text-blue-300 mb-4">
                                This URL contains a playlist with <strong>{{ playlistInfo.playlist_count }}</strong> video{{ playlistInfo.playlist_count !== 1 ? 's' : '' }}.
                                <span v-if="playlistInfo.first_video_title"> First video: "{{ playlistInfo.first_video_title }}"</span>
                            </p>
                            <div class="flex flex-wrap gap-3">
                                <button 
                                    type="button"
                                    @click="startFirstVideoDownload"
                                    :disabled="isSubmitting"
                                    class="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded-lg font-medium transition-colors duration-200 flex items-center space-x-2 disabled:opacity-50"
                                >
                                    <svg v-if="!isSubmitting" class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.828 14.828a4 4 0 01-5.656 0M9 10h1m4 0h1m-6 4h1m4 0h1m6-10V7a3 3 0 00-3-3H6a3 3 0 00-3 3v1"/>
                                    </svg>
                                    <svg v-else class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                        <path class="opacity-75" fill="currentColor" d="m4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                    </svg>
                                    <span>{{ isSubmitting ? 'Starting Download...' : 'Download First Video Only' }}</span>
                                </button>
                                <button 
                                    type="button"
                                    @click="startPlaylistDownload"
                                    :disabled="isSubmitting"
                                    class="bg-green-500 hover:bg-green-600 text-white px-4 py-2 rounded-lg font-medium transition-colors duration-200 flex items-center space-x-2 disabled:opacity-50"
                                >
                                    <svg v-if="!isSubmitting" class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/>
                                    </svg>
                                    <svg v-else class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                        <path class="opacity-75" fill="currentColor" d="m4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                    </svg>
                                    <span>{{ isSubmitting ? 'Starting Downloads...' : 'Download Entire Playlist (' + playlistInfo.playlist_count + ' videos)' }}</span>
                                </button>
                            </div>
                        </div>
                        
                        <div>
                            <label for="type" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Type</label>
                            <select 
                                id="type"
                                v-model="newDownload.type" 
                                @change="updateDefaultFormat"
                                class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors"
                            >
                                <option value="video">Video</option>
                                <option value="audio">Audio Only</option>
                            </select>
                        </div>
                        <div>
                            <label for="format" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Format</label>
                            <select 
                                id="format"
                                v-model="newDownload.format" 
                                class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors"
                            >
                                <option v-if="newDownload.type === 'video'" value="mp4">MP4</option>
                                <option v-if="newDownload.type === 'video'" value="webm">WebM</option>
                                <option v-if="newDownload.type === 'video'" value="mkv">MKV</option>
                                <option v-if="newDownload.type === 'audio'" value="mp3">MP3</option>
                                <option v-if="newDownload.type === 'audio'" value="m4a">M4A</option>
                                <option v-if="newDownload.type === 'audio'" value="wav">WAV</option>
                                <option v-if="newDownload.type === 'audio'" value="flac">FLAC</option>
                            </select>
                        </div>
                        <div v-if="newDownload.type === 'video'">
                            <label for="quality" class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Quality</label>
                            <select 
                                id="quality"
                                v-model="newDownload.quality" 
                                class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors"
                            >
                                <option value="best">Best Available</option>
                                <option value="4K">4K (2160p)</option>
                                <option value="2K">2K (1440p)</option>
                                <option value="1080p">1080p</option>
                                <option value="720p">720p</option>
                                <option value="480p">480p</option>
                                <option value="360p">360p</option>
                            </select>
                        </div>
                    </div>
                    
                    <div class="flex justify-end">
                        <button 
                            v-if="!playlistInfo || !playlistInfo.is_playlist"
                            type="submit" 
                            :disabled="isSubmitting || isValidatingUrl || !newDownload.url"
                            class="btn-primary text-white px-8 py-3 rounded-xl font-medium transition-all duration-200 flex items-center space-x-2 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            <svg v-if="!isSubmitting && !isValidatingUrl" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                            </svg>
                            <svg v-else class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
                                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                <path class="opacity-75" fill="currentColor" d="m4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                            </svg>
                            <span>{{ isSubmitting || isValidatingUrl ? 'Processing URL...' : 'Start Download' }}</span>
                        </button>
                    </div>
                </form>
            </div>
        </div>

        <!-- Status message -->
        <div v-if="statusMessage" class="max-w-4xl mx-auto mb-8 p-4 rounded-xl" :class="statusMessage.type === 'success' ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'">
            {{ statusMessage.text }}
        </div>

        <!-- Queue Section -->
        <div v-if="queuedDownloads.length > 0 || sections.queued.forceShow" class="max-w-4xl mx-auto mt-8 bg-white dark:bg-slate-800 rounded-2xl shadow-2xl card-shadow">
            <div @click="toggleSection('queued')" class="section-header flex items-center justify-between p-6 border-b border-slate-200 dark:border-slate-700">
                <div class="flex items-center">
                    <div class="p-3 bg-gradient-to-r from-amber-500 to-orange-600 rounded-2xl shadow-lg mr-4">
                        <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
                        </svg>
                    </div>
                    <h2 class="text-lg sm:text-2xl font-bold text-slate-800 dark:text-white">Queue</h2>
                </div>
                <div class="flex items-center space-x-3">
                    <span class="text-sm text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-700 px-3 py-1 rounded">
                        {{ queuedDownloads.length }}
                    </span>
                    <button v-if="queuedDownloads.length > 0" @click.stop="clearAllQueued()" class="text-xs bg-red-500 hover:bg-red-600 text-white px-2 py-1 rounded transition-colors duration-200" title="Clear All Queued">
                        Clear All
                    </button>
                    <svg class="w-5 h-5 text-slate-400 transition-transform duration-300" :class="{ 'rotate-180': sections.queued.expanded }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                    </svg>
                </div>
            </div>
            
            <div class="accordion-content" :class="sections.queued.expanded ? 'accordion-expanded' : 'accordion-collapsed'">
                <div class="p-4 sm:p-6 download-list-container">
                    <div class="space-y-3" :class="queuedDownloads.length > 10 ? 'download-list-scrollable' : ''">
                    <div v-for="download in queuedDownloads" :key="download.id" class="download-item group relative overflow-hidden border border-amber-200 dark:border-amber-700 rounded-xl p-4 hover:shadow-lg transition-all duration-300 bg-gradient-to-r from-amber-50 to-orange-50 dark:from-amber-900 dark:to-orange-900">
                        <div class="flex justify-between items-start mb-2">
                            <div class="flex-1 min-w-0 pr-4">
                                <div class="flex items-center mb-1">
                                    <svg v-if="download.type === 'video'" class="w-4 h-4 text-blue-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/>
                                    </svg>
                                    <svg v-if="download.type === 'audio'" class="w-4 h-4 text-green-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"/>
                                    </svg>
                                    <h3 class="font-semibold text-slate-800 dark:text-white truncate text-sm">{{ download.title && download.title !== download.url ? download.title : 'Loading...' }}</h3>
                                </div>
                                <p class="text-xs text-slate-700 dark:text-slate-300 truncate">{{ download.url }}</p>
                                <p class="text-xs text-slate-600 dark:text-slate-400">{{ download.type }} • {{ download.format }} {{ download.quality ? '• ' + download.quality : '' }}</p>
                                <p class="text-xs text-slate-600 dark:text-slate-400">Added: {{ formatDate(download.created_at) }}</p>
                            </div>
                            <div class="flex items-center space-x-2">
                                <span class="status-badge status-queued">queued</span>
                                <button @click="deleteDownload(download.id)" class="p-1 text-red-500 hover:text-red-700">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                                    </svg>
                                </button>
                            </div>
                        </div>
                    </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Active Downloads Section -->
        <div v-if="activeDownloads.length > 0 || sections.downloading.forceShow" class="max-w-4xl mx-auto mt-8 bg-white dark:bg-slate-800 rounded-2xl shadow-2xl card-shadow">
            <div @click="toggleSection('downloading')" class="section-header flex items-center justify-between p-6 border-b border-slate-200 dark:border-slate-700">
                <div class="flex items-center">
                    <div class="p-3 bg-gradient-to-r from-blue-500 to-indigo-600 rounded-2xl shadow-lg mr-4">
                        <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/>
                        </svg>
                    </div>
                    <h2 class="text-lg sm:text-2xl font-bold text-slate-800 dark:text-white">Active Downloads</h2>
                </div>
                <div class="flex items-center space-x-3">
                    <span class="text-sm text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-700 px-3 py-1 rounded">
                        {{ activeDownloads.length }}
                    </span>
                    <svg class="w-5 h-5 text-slate-400 transition-transform duration-300" :class="{ 'rotate-180': sections.downloading.expanded }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                    </svg>
                </div>
            </div>
            
            <div class="accordion-content" :class="sections.downloading.expanded ? 'accordion-expanded' : 'accordion-collapsed'">
                <div class="p-4 sm:p-6 download-list-container">
                    <div class="space-y-3" :class="activeDownloads.length > 10 ? 'download-list-scrollable' : ''">
                    <div v-for="download in activeDownloads" :key="download.id" class="download-item group relative overflow-hidden border border-blue-200 dark:border-blue-700 rounded-xl p-4 hover:shadow-lg transition-all duration-300 bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900 dark:to-indigo-900">
                        <div class="flex flex-col sm:flex-row sm:justify-between sm:items-start mb-2 space-y-2 sm:space-y-0">
                            <div class="flex-1 min-w-0 pr-4">
                                <div class="flex items-center mb-1">
                                    <svg v-if="download.type === 'video'" class="w-4 h-4 text-blue-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/>
                                    </svg>
                                    <svg v-if="download.type === 'audio'" class="w-4 h-4 text-green-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"/>
                                    </svg>
                                    <h3 class="font-semibold text-slate-800 dark:text-white truncate text-sm">{{ download.title && download.title !== download.url ? download.title : 'Loading...' }}</h3>
                                </div>
                                <div class="progress-bar">
                                    <div class="progress-fill" :style="{ width: (download.progress ? download.progress.percentage : 0) + '%' }"></div>
                                </div>
                                <p class="text-xs text-slate-700 dark:text-slate-300 truncate">{{ download.url }}</p>
                                <p class="text-xs text-slate-600 dark:text-slate-400">{{ download.type }} • {{ download.format }} {{ download.quality ? '• ' + download.quality : '' }} • {{ download.progress ? Math.round(download.progress.percentage) : 0 }}%</p>
                                <p v-if="download.progress && download.progress.speed" class="text-xs text-slate-600 dark:text-slate-400">Speed: {{ download.progress.speed }}{{ download.progress.eta ? ' • ETA: ' + download.progress.eta : '' }}</p>
                                <p v-if="download.status_message" class="text-xs text-slate-600 dark:text-slate-400">Status: {{ download.status_message }}</p>
                            </div>
                            <div class="flex items-center space-x-2 ">
                                <span class="status-badge status-downloading">downloading</span>
                                <button @click="pauseDownload(download.id)" class="p-1 text-yellow-500 hover:text-yellow-700">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6"/>
                                    </svg>
                                </button>
                                <button @click="deleteDownload(download.id)" class="p-1 text-red-500 hover:text-red-700">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                                    </svg>
                                </button>
                            </div>
                        </div>
                    </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Post Processing Section -->
        <div v-if="processingDownloads.length > 0 || sections.processing.forceShow" class="max-w-4xl mx-auto mt-8 bg-white dark:bg-slate-800 rounded-2xl shadow-2xl card-shadow">
            <div @click="toggleSection('processing')" class="section-header flex items-center justify-between p-6 border-b border-slate-200 dark:border-slate-700">
                <div class="flex items-center">
                    <div class="p-3 bg-gradient-to-r from-purple-500 to-violet-600 rounded-2xl shadow-lg mr-4">
                        <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"/>
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/>
                        </svg>
                    </div>
                    <h2 class="text-lg sm:text-2xl font-bold text-slate-800 dark:text-white">Post Processing</h2>
                </div>
                <div class="flex items-center space-x-3">
                    <span class="text-sm text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-700 px-3 py-1 rounded">
                        {{ processingDownloads.length }}
                    </span>
                    <svg class="w-5 h-5 text-slate-400 transition-transform duration-300" :class="{ 'rotate-180': sections.processing.expanded }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                    </svg>
                </div>
            </div>
            
            <div class="accordion-content" :class="sections.processing.expanded ? 'accordion-expanded' : 'accordion-collapsed'">
                <div class="p-4 sm:p-6 download-list-container">
                    <div class="space-y-3" :class="processingDownloads.length > 10 ? 'download-list-scrollable' : ''">
                    <div v-for="download in processingDownloads" :key="download.id" class="download-item group relative overflow-hidden border border-purple-200 dark:border-purple-700 rounded-xl p-4 hover:shadow-lg transition-all duration-300 bg-gradient-to-r from-purple-100 to-violet-100 dark:from-purple-800 dark:to-violet-800">
                        <div class="flex flex-col space-y-3 sm:flex-row sm:justify-between sm:items-start sm:space-y-0 mb-2">
                            <div class="flex-1 min-w-0 sm:pr-4">
                                <div class="flex items-center mb-1">
                                    <svg v-if="download.type === 'video'" class="w-4 h-4 text-blue-300 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/>
                                    </svg>
                                    <svg v-if="download.type === 'audio'" class="w-4 h-4 text-green-300 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"/>
                                    </svg>
                                    <h3 class="font-semibold text-white dark:text-purple-100 truncate text-sm">{{ download.title && download.title !== download.url ? download.title : 'Loading...' }}</h3>
                                </div>
                                <div class="processing-indicator">
                                    <div class="processing-spinner"></div>
                                    <span class="text-sm text-purple-200 dark:text-purple-300 font-medium">Converting with FFmpeg</span>
                                    <div class="processing-dots">
                                        <div class="processing-dot"></div>
                                        <div class="processing-dot"></div>
                                        <div class="processing-dot"></div>
                                    </div>
                                </div>
                                <div class="space-y-1">
                                    <p class="text-xs text-purple-200 dark:text-purple-300">{{ download.type }} • {{ download.format }} {{ download.quality ? '• ' + download.quality : '' }}</p>
                                    <p v-if="download.status_message" class="text-xs text-purple-200 dark:text-purple-300">Status: {{ download.status_message }}</p>
                                </div>
                            </div>
                            <div class="flex flex-wrap items-center gap-3 justify-start sm:justify-end">
                                <span class="status-badge status-post-processing">post-processing</span>
                                <button @click="deleteDownload(download.id)" class="p-1 text-red-500 hover:text-red-700">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                                    </svg>
                                </button>
                            </div>
                        </div>
                    </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Completed Downloads -->
        <div v-if="completedDownloads.length > 0 || sections.completed.forceShow" class="max-w-4xl mx-auto mt-8 bg-white dark:bg-slate-800 rounded-2xl shadow-2xl card-shadow">
            <div @click="toggleSection('completed')" class="section-header flex items-center justify-between p-6 border-b border-slate-200 dark:border-slate-700">
                <div class="flex items-center">
                    <div class="p-3 bg-gradient-to-r from-emerald-500 to-teal-600 rounded-2xl shadow-lg mr-4">
                        <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
                        </svg>
                    </div>
                    <h2 class="text-lg sm:text-2xl font-bold text-slate-800 dark:text-white">Completed Downloads</h2>
                </div>
                <div class="flex items-center space-x-3">
                    <span class="text-sm text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-700 px-3 py-1 rounded">
                        {{ completedDownloads.length }}
                    </span>
                    <button v-if="completedDownloads.length > 0" @click.stop="deleteAllCompleted()" class="text-xs bg-red-500 hover:bg-red-600 text-white px-2 py-1 rounded transition-colors duration-200" title="Delete All Completed">
                        Delete All
                    </button>
                    <svg class="w-5 h-5 text-slate-400 transition-transform duration-300" :class="{ 'rotate-180': sections.completed.expanded }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                    </svg>
                </div>
            </div>
            
            <div class="accordion-content" :class="sections.completed.expanded ? 'accordion-expanded' : 'accordion-collapsed'">
                <div class="p-4 sm:p-6 download-list-container">
                    <div class="space-y-3" :class="completedDownloads.length > 10 ? 'download-list-scrollable' : ''">
                    <div v-for="download in completedDownloads" :key="download.id" class="download-item group relative overflow-hidden border border-green-200 dark:border-green-700 rounded-xl p-3 sm:p-4 hover:shadow-lg transition-all duration-300 bg-gradient-to-r from-green-100 to-emerald-100 dark:from-green-800 dark:to-emerald-800">
                        <div class="flex flex-col space-y-3 sm:flex-row sm:justify-between sm:items-start sm:space-y-0 mb-2">
                            <div class="flex-1 min-w-0 sm:pr-4">
                                <div class="flex items-center mb-1">
                                    <svg v-if="download.type === 'video'" class="w-4 h-4 text-blue-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/>
                                    </svg>
                                    <svg v-if="download.type === 'audio'" class="w-4 h-4 text-green-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"/>
                                    </svg>
                                    <h3 class="font-semibold text-slate-800 dark:text-white truncate text-sm">{{ download.title && download.title !== download.url ? download.title : 'Loading...' }}</h3>
                                </div>
                                <div class="space-y-1">
                                    <p class="text-xs text-slate-700 dark:text-slate-300 truncate break-all">{{ download.url }}</p>
                                    <p class="text-xs text-slate-600 dark:text-slate-400">{{ download.type }} • {{ download.format }} {{ download.quality ? '• ' + download.quality : '' }}</p>
                                    <p class="text-xs text-slate-600 dark:text-slate-400">Completed: {{ formatDate(download.completed_at) }}</p>
                                </div>
                            </div>
                            <div class="flex flex-wrap items-center gap-3 justify-start sm:justify-end">
                                <span class="status-badge status-completed">completed</span>
                                <a v-if="download.output_path" :href="'/api/downloads/' + download.id + '/download'" download class="btn-sm bg-blue-500 hover:bg-blue-600 text-white px-3 py-1 rounded text-xs transition-colors duration-200 flex items-center space-x-1" title="Download File">
                                    <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                                    </svg>
                                    <span>Download</span>
                                </a>
                                <button @click="deleteDownload(download.id)" class="opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 text-red-500 hover:text-red-700" title="Delete">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                                    </svg>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        </div>

        <!-- Failed Downloads -->
        <div v-if="failedDownloads.length > 0 || sections.failed.forceShow" class="max-w-4xl mx-auto mt-12 bg-white dark:bg-slate-800 rounded-2xl shadow-2xl card-shadow">
            <div @click="toggleSection('failed')" class="section-header flex items-center justify-between p-6 border-b border-slate-200 dark:border-slate-700">
                <div class="flex items-center">
                    <div class="p-3 bg-gradient-to-r from-red-500 to-rose-600 rounded-2xl shadow-lg mr-4">
                        <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
                        </svg>
                    </div>
                    <h2 class="text-lg sm:text-2xl font-bold text-slate-800 dark:text-white">Failed Downloads</h2>
                </div>
                <div class="flex items-center space-x-3">
                    <span class="text-sm text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-700 px-3 py-1 rounded">
                        {{ failedDownloads.length }}
                    </span>
                    <button v-if="failedDownloads.length > 0" @click.stop="clearAllFailed()" class="text-xs bg-red-500 hover:bg-red-600 text-white px-2 py-1 rounded transition-colors duration-200" title="Clear All Failed">
                        Clear All
                    </button>
                    <svg class="w-5 h-5 text-slate-400 transition-transform duration-300" :class="{ 'rotate-180': sections.failed.expanded }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
                    </svg>
                </div>
            </div>
            
            <div class="accordion-content" :class="sections.failed.expanded ? 'accordion-expanded' : 'accordion-collapsed'">
                <div class="p-4 sm:p-6 download-list-container">
                    <div class="space-y-3" :class="failedDownloads.length > 10 ? 'download-list-scrollable' : ''">
                    <div v-for="download in failedDownloads" :key="download.id" class="download-item group relative overflow-hidden border border-red-200 dark:border-red-700 rounded-xl p-3 sm:p-4 hover:shadow-lg transition-all duration-300 bg-gradient-to-r from-red-50 to-rose-50 dark:from-red-900 dark:to-rose-900">
                        <div class="flex flex-col space-y-3 sm:flex-row sm:justify-between sm:items-start sm:space-y-0 mb-2">
                            <div class="flex-1 min-w-0 sm:pr-4">
                                <div class="flex items-center mb-1">
                                    <svg v-if="download.type === 'video'" class="w-4 h-4 text-blue-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/>
                                    </svg>
                                    <svg v-if="download.type === 'audio'" class="w-4 h-4 text-green-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3"/>
                                    </svg>
                                    <h3 class="font-semibold text-slate-800 dark:text-white truncate text-sm">{{ download.title && download.title !== download.url ? download.title : 'Loading...' }}</h3>
                                </div>
                                <div class="space-y-1">
                                    <p class="text-xs text-slate-700 dark:text-slate-300 truncate break-all">{{ download.url }}</p>
                                    <p class="text-xs text-slate-600 dark:text-slate-400">{{ download.type }} • {{ download.format }} {{ download.quality ? '• ' + download.quality : '' }}</p>
                                    <p v-if="download.error_message" class="text-xs text-red-600 dark:text-red-400">Error: {{ download.error_message }}</p>
                                    <p class="text-xs text-slate-600 dark:text-slate-400">Failed: {{ formatDate(download.error_at || download.created_at) }}</p>
                                </div>
                            </div>
                            <div class="flex flex-wrap items-center gap-3 justify-start sm:justify-end ">
                                <span class="status-badge status-failed">failed</span>
                                <button @click="retryDownload(download.id)" class="p-1 text-blue-500 hover:text-blue-700">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                                    </svg>
                                </button>
                                <button @click="deleteDownload(download.id)" class="p-1 text-red-500 hover:text-red-700">
                                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                                    </svg>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Settings Modal -->
        <div v-if="showSettings" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50" @click.self="showSettings = false">
            <div class="bg-white dark:bg-slate-800 rounded-2xl shadow-2xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
                <div class="p-6 border-b border-slate-200 dark:border-slate-700">
                    <div class="flex items-center justify-between">
                        <h2 class="text-lg sm:text-2xl font-bold text-slate-800 dark:text-white">Settings</h2>
                        <button @click="showSettings = false" class="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700 transition-colors">
                            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
                            </svg>
                        </button>
                    </div>
                </div>
                
                <div class="p-6 space-y-6">
                    <form @submit.prevent="saveSettings" class="space-y-6">
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <div class="md:col-span-2">
                                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Download Path</label>
                                <input v-model="settings.download_path" type="text" class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors" required>
                            </div>
                            
                            <div>
                                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Max Concurrent Downloads</label>
                                <select v-model.number="settings.max_concurrent_downloads" class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors">
                                    <option value="1">1</option>
                                    <option value="2">2</option>
                                    <option value="3">3</option>
                                    <option value="4">4</option>
                                    <option value="5">5</option>
                                </select>
                            </div>
                            
                            <div>
                                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">File Expiry</label>
                                <select v-model.number="settings.completed_file_expiry_hours" class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors">
                                    <option value="0">Never</option>
                                    <option value="1">1 Hour</option>
                                    <option value="24">24 Hours</option>
                                    <option value="48">48 Hours</option>
                                    <option value="72">72 Hours</option>
                                </select>
                            </div>
                            
                            <div class="md:col-span-2">
                                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">yt-dlp Path</label>
                                <input v-model="settings.yt_dlp_path" type="text" class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors" required>
                            </div>
                            
                            <div class="md:col-span-2">
                                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">FFmpeg Path</label>
                                <input v-model="settings.ffmpeg_path" type="text" class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors" required>
                            </div>
                            
                            <div>
                                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Default Video Format</label>
                                <select v-model="settings.default_video_format" class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors">
                                    <option value="mp4">MP4</option>
                                    <option value="webm">WebM</option>
                                    <option value="mkv">MKV</option>
                                </select>
                            </div>
                            
                            <div>
                                <label class="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-2">Default Audio Format</label>
                                <select v-model="settings.default_audio_format" class="w-full px-4 py-3 border border-slate-300 dark:border-slate-600 rounded-xl focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-700 dark:text-white transition-colors">
                                    <option value="mp3">MP3</option>
                                    <option value="m4a">M4A</option>
                                    <option value="wav">WAV</option>
                                    <option value="flac">FLAC</option>
                                </select>
                            </div>
                            
                            <div class="md:col-span-2">
                                <label class="flex items-center">
                                    <input v-model="settings.verbose_logging" type="checkbox" class="rounded border-slate-300 dark:border-slate-600 text-blue-600 focus:ring-blue-500 dark:bg-slate-700">
                                    <span class="ml-2 text-sm font-medium text-slate-700 dark:text-slate-300">Enable Verbose Logging</span>
                                </label>
                            </div>
                            
                            <!-- yt-dlp Update Section -->
                            <div class="md:col-span-2 bg-slate-50 dark:bg-slate-700 rounded-xl p-4">
                                <div class="flex items-center justify-between mb-4">
                                    <h3 class="text-lg font-semibold text-slate-800 dark:text-white">yt-dlp Updates</h3>
                                    <button type="button" @click="checkForUpdates" :disabled="isCheckingUpdates" class="bg-blue-500 hover:bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors duration-200 disabled:opacity-50">
                                        {{ isCheckingUpdates ? 'Checking...' : 'Check for Updates' }}
                                    </button>
                                </div>
                                
                                <div v-if="updateInfo" class="space-y-2 text-sm">
                                    <div class="flex justify-between">
                                        <span class="text-slate-600 dark:text-slate-400">Current Version:</span>
                                        <code class="bg-slate-200 dark:bg-slate-600 px-2 py-1 rounded text-xs">{{ updateInfo.current_version }}</code>
                                    </div>
                                    <div class="flex justify-between">
                                        <span class="text-slate-600 dark:text-slate-400">Latest Version:</span>
                                        <code class="bg-slate-200 dark:bg-slate-600 px-2 py-1 rounded text-xs">{{ updateInfo.latest_version }}</code>
                                    </div>
                                    <div v-if="updateInfo.update_available" class="flex justify-between items-center mt-4 p-3 bg-amber-50 dark:bg-amber-900 border border-amber-200 dark:border-amber-700 rounded-lg">
                                        <div class="flex items-center">
                                            <svg class="w-5 h-5 text-amber-600 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"/>
                                            </svg>
                                            <span class="text-amber-800 dark:text-amber-200 font-medium">Update Available!</span>
                                        </div>
                                        <button type="button" @click="updateYtDlp" :disabled="isUpdating" class="bg-green-500 hover:bg-green-600 text-white px-3 py-1 rounded text-sm font-medium transition-colors duration-200 disabled:opacity-50">
                                            {{ isUpdating ? 'Updating...' : 'Update Now' }}
                                        </button>
                                    </div>
                                    <div v-else class="flex items-center mt-4 p-3 bg-green-50 dark:bg-green-900 border border-green-200 dark:border-green-700 rounded-lg">
                                        <svg class="w-5 h-5 text-green-600 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                        </svg>
                                        <span class="text-green-800 dark:text-green-200 font-medium">yt-dlp is up to date!</span>
                                    </div>
                                    <div class="text-xs text-slate-500 dark:text-slate-400 mt-2">
                                        Last checked: {{ formatDate(updateInfo.last_checked) }}
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <div class="flex justify-end space-x-4 pt-6 border-t border-slate-200 dark:border-slate-700">
                            <button type="button" @click="showSettings = false" class="px-6 py-3 border border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-300 rounded-xl hover:bg-slate-50 dark:hover:bg-slate-700 transition-colors">
                                Cancel
                            </button>
                            <button type="submit" :disabled="isSavingSettings" class="btn-primary text-white px-6 py-3 rounded-xl font-medium transition-all duration-200 disabled:opacity-50">
                                {{ isSavingSettings ? 'Saving...' : 'Save Settings' }}
                            </button>
                        </div>
                    </form>
                </div>
                    </div>
                </div>
            </div>
        </div>

    </div>

    <!-- Footer -->
    <footer class="bg-white dark:bg-slate-800 border-t border-slate-200 dark:border-slate-700 mt-12">
        <div class="container mx-auto px-4 py-6">
            <div class="flex flex-col md:flex-row justify-between items-center space-y-4 md:space-y-0">
                <div class="text-center md:text-left">
                    <p class="text-sm text-slate-600 dark:text-slate-400">
                        GoGetMedia - A WebUI for yt-dlp
                    </p>
                </div>
            </div>
        </div>
    </footer>

    <script>
        const { createApp } = Vue;
        
        createApp({
            data() {
                return {
                    downloads: [],
                    newDownload: {
                        url: '',
                        type: 'video',
                        quality: 'best',
                        format: 'mp4'
                    },
                    isSubmitting: false,
                    isDarkMode: false,
                    isConnected: false,
                    statusMessage: null,
                    playlistInfo: null,
                    isValidatingUrl: false,
                    validationTimeout: null,
                    showSettings: false,
                    isSavingSettings: false,
                    isCheckingUpdates: false,
                    isUpdating: false,
                    updateInfo: null,
                    settings: {
                        download_path: '',
                        max_concurrent_downloads: 3,
                        yt_dlp_path: '',
                        ffmpeg_path: '',
                        port: 8080,
                        default_video_format: 'mp4',
                        default_audio_format: 'mp3',
                        verbose_logging: false,
                        completed_file_expiry_hours: 72
                    },
                    versions: {
                        yt_dlp: '',
                        ffmpeg: ''
                    },
                    sections: {
                        completed: { expanded: true, forceShow: true },
                        downloading: { expanded: true, forceShow: true },
                        queued: { expanded: true, forceShow: true },
                        processing: { expanded: true, forceShow: true },
                        failed: { expanded: true, forceShow: true }
                    }
                }
            },
            
            computed: {
                queuedDownloads() {
                    return this.downloads.filter(d => d.status === 'queued').sort((a, b) => {
                        const aDate = new Date(a.created_at);
                        const bDate = new Date(b.created_at);
                        const timeDiff = aDate - bDate;
                        // If dates are the same, use ID as secondary sort for stability
                        return timeDiff !== 0 ? timeDiff : a.id.localeCompare(b.id);
                    });
                },
                
                activeDownloads() {
                    return this.downloads.filter(d => d.status === 'downloading').sort((a, b) => {
                        const aDate = new Date(a.started_at || a.created_at);
                        const bDate = new Date(b.started_at || b.created_at);
                        const timeDiff = bDate - aDate;
                        // If dates are the same, use ID as secondary sort for stability
                        return timeDiff !== 0 ? timeDiff : a.id.localeCompare(b.id);
                    });
                },
                
                processingDownloads() {
                    return this.downloads.filter(d => d.status === 'post-processing').sort((a, b) => {
                        const aDate = new Date(a.started_at || a.created_at);
                        const bDate = new Date(b.started_at || b.created_at);
                        const timeDiff = bDate - aDate;
                        // If dates are the same, use ID as secondary sort for stability
                        return timeDiff !== 0 ? timeDiff : a.id.localeCompare(b.id);
                    });
                },
                
                completedDownloads() {
                    return this.downloads.filter(d => 
                        d.status === 'completed' || 
                        d.status === 'already_exists'
                    ).sort((a, b) => {
                        const aDate = new Date(a.completed_at || a.created_at);
                        const bDate = new Date(b.completed_at || b.created_at);
                        const timeDiff = bDate - aDate;
                        // If dates are the same, use ID as secondary sort for stability
                        return timeDiff !== 0 ? timeDiff : a.id.localeCompare(b.id);
                    });
                },
                
                failedDownloads() {
                    return this.downloads.filter(d => 
                        d.status === 'failed' || 
                        d.status === 'error'
                    ).sort((a, b) => {
                        const aDate = new Date(a.error_at || a.created_at);
                        const bDate = new Date(b.error_at || b.created_at);
                        const timeDiff = bDate - aDate;
                        // If dates are the same, use ID as secondary sort for stability
                        return timeDiff !== 0 ? timeDiff : a.id.localeCompare(b.id);
                    });
                }
            },
            
            methods: {
                toggleSection(section) {
                    this.sections[section].expanded = !this.sections[section].expanded;
                },
                
                toggleDarkMode() {
                    this.isDarkMode = !this.isDarkMode;
                    if (this.isDarkMode) {
                        document.documentElement.classList.add('dark');
                        localStorage.setItem('darkMode', 'true');
                    } else {
                        document.documentElement.classList.remove('dark');
                        localStorage.setItem('darkMode', 'false');
                    }
                },
                
                checkDarkMode() {
                    const savedMode = localStorage.getItem('darkMode');
                    if (savedMode === 'true') {
                        this.isDarkMode = true;
                        document.documentElement.classList.add('dark');
                    } else {
                        this.isDarkMode = false;
                        document.documentElement.classList.remove('dark');
                    }
                },
                
                formatDate(dateString) {
                    if (!dateString) return 'Unknown';
                    const date = new Date(dateString);
                    return date.toLocaleString();
                },
                
                updateDefaultFormat() {
                    // Set default formats based on type
                    if (this.newDownload.type === 'video') {
                        this.newDownload.format = 'mp4';
                    } else if (this.newDownload.type === 'audio') {
                        this.newDownload.format = 'mp3';
                    }
                },
                
                async validateUrl() {
                    // Clear validation timeout if it exists
                    if (this.validationTimeout) {
                        clearTimeout(this.validationTimeout);
                    }
                    
                    // Clear previous playlist info
                    this.playlistInfo = null;
                    
                    // Don't validate empty URLs
                    if (!this.newDownload.url.trim()) {
                        return;
                    }
                    
                    // Only validate YouTube URLs for playlist detection
                    // Non-YouTube URLs will be validated only when submitted
                    if (!this.newDownload.url.includes('youtube.com') && !this.newDownload.url.includes('youtu.be')) {
                        return;
                    }
                    
                    // Debounce validation to avoid too many API calls
                    this.validationTimeout = setTimeout(async () => {
                        this.isValidatingUrl = true;
                        
                        try {
                            const response = await fetch('/api/validate', {
                                method: 'POST',
                                headers: {
                                    'Content-Type': 'application/json'
                                },
                                body: JSON.stringify({
                                    url: this.newDownload.url,
                                    type: this.newDownload.type,
                                    quality: this.newDownload.quality,
                                    format: this.newDownload.format
                                })
                            });
                            
                            if (response.ok) {
                                const result = await response.json();
                                if (result.valid) {
                                    this.playlistInfo = result;
                                }
                            }
                        } catch (error) {
                            console.error('URL validation error:', error);
                        } finally {
                            this.isValidatingUrl = false;
                        }
                    }, 300); // Wait 300ms after user stops typing
                },
                
                async startFirstVideoDownload() {
                    if (!this.playlistInfo || !this.playlistInfo.is_playlist) {
                        return;
                    }
                    
                    this.isSubmitting = true;
                    this.statusMessage = null;
                    
                    try {
                        const response = await fetch('/api/downloads/first-video', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json'
                            },
                            body: JSON.stringify({
                                url: this.newDownload.url,
                                type: this.newDownload.type,
                                quality: this.newDownload.quality,
                                format: this.newDownload.format
                            })
                        });
                        
                        if (response.ok) {
                            this.statusMessage = { type: 'success', text: 'First video download started!' };
                            this.newDownload.url = '';
                            this.playlistInfo = null;
                            await this.loadDownloads();
                        } else {
                            const error = await response.text();
                            this.statusMessage = { type: 'error', text: 'Error: ' + error };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    } finally {
                        this.isSubmitting = false;
                        setTimeout(() => {
                            this.statusMessage = null;
                        }, 5000);
                    }
                },
                
                async startPlaylistDownload() {
                    if (!this.playlistInfo || !this.playlistInfo.is_playlist) {
                        return;
                    }
                    
                    this.isSubmitting = true;
                    this.statusMessage = null;
                    
                    try {
                        const response = await fetch('/api/downloads/playlist', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json'
                            },
                            body: JSON.stringify({
                                url: this.newDownload.url,
                                type: this.newDownload.type,
                                quality: this.newDownload.quality,
                                format: this.newDownload.format
                            })
                        });
                        
                        if (response.ok) {
                            this.statusMessage = { type: 'success', text: 'Playlist download started! (' + this.playlistInfo.playlist_count + ' videos)' };
                            this.newDownload.url = '';
                            this.playlistInfo = null;
                            await this.loadDownloads();
                        } else {
                            const error = await response.text();
                            this.statusMessage = { type: 'error', text: 'Error: ' + error };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    } finally {
                        this.isSubmitting = false;
                        setTimeout(() => {
                            this.statusMessage = null;
                        }, 5000);
                    }
                },
                
                async validateUrlImmediate() {
                    // Immediate validation without debounce for form submission
                    this.playlistInfo = null;
                    
                    if (!this.newDownload.url.trim()) {
                        return;
                    }
                    
                    // Only validate YouTube URLs for playlist detection
                    if (!this.newDownload.url.includes('youtube.com') && !this.newDownload.url.includes('youtu.be')) {
                        return;
                    }
                    
                    this.isValidatingUrl = true;
                    
                    try {
                        const response = await fetch('/api/validate', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json'
                            },
                            body: JSON.stringify({
                                url: this.newDownload.url,
                                type: this.newDownload.type,
                                quality: this.newDownload.quality,
                                format: this.newDownload.format
                            })
                        });
                        
                        if (response.ok) {
                            const result = await response.json();
                            if (result.valid && result.is_playlist) {
                                this.playlistInfo = result;
                            }
                        }
                    } catch (error) {
                        console.error('URL validation error:', error);
                    } finally {
                        this.isValidatingUrl = false;
                    }
                },
                
                async handleFormSubmit() {
                    // Form submission should start download for regular URLs
                    // If it's a playlist, user must choose explicitly with playlist buttons
                    
                    // If it's a playlist, just validate without auto-downloading
                    if (this.playlistInfo && this.playlistInfo.is_playlist) {
                        return;
                    }
                    
                    // Check URL directly for playlist indicators and validate if needed
                    if (this.newDownload.url.includes('list=') || this.newDownload.url.includes('playlist')) {
                        // If we detect playlist indicators but don't have playlist info yet, 
                        // trigger validation immediately to show playlist options
                        if (!this.playlistInfo) {
                            await this.validateUrlImmediate();
                        }
                        return;
                    }
                    
                    // For regular URLs, start the download
                    if (this.newDownload.url.trim()) {
                        await this.startDownload();
                    }
                },
                
                checkForDuplicate() {
                    const currentUrl = this.newDownload.url.trim();
                    const currentType = this.newDownload.type;
                    const currentQuality = this.newDownload.quality;
                    const currentFormat = this.newDownload.format;
                    
                    // Check all downloads for exact match
                    const allDownloads = [...this.downloads];
                    
                    for (const download of allDownloads) {
                        if (download.url === currentUrl && 
                            download.type === currentType && 
                            download.quality === currentQuality && 
                            download.format === currentFormat) {
                            
                            // Check status to give appropriate message
                            if (download.status === 'completed' || download.status === 'already_exists') {
                                return { isDuplicate: true, message: 'This URL has already been downloaded with the same quality and format.' };
                            } else if (download.status === 'downloading' || download.status === 'queued' || download.status === 'post-processing') {
                                return { isDuplicate: true, message: 'This URL is already being downloaded with the same quality and format.' };
                            } else if (download.status === 'failed') {
                                return { isDuplicate: true, message: 'This URL was previously attempted. You can retry by removing the failed download first.' };
                            }
                        }
                    }
                    
                    return { isDuplicate: false, message: null };
                },
                
                async startDownload() {
                    // Don't auto-download if it's a playlist - user must choose explicitly
                    if (this.playlistInfo && this.playlistInfo.is_playlist) {
                        return;
                    }
                    
                    // Also check URL directly for playlist indicators as a fallback
                    if (this.newDownload.url.includes('list=') || this.newDownload.url.includes('playlist')) {
                        this.statusMessage = { type: 'error', text: 'This appears to be a playlist URL. Please wait for validation to complete and use the playlist-specific download buttons.' };
                        return;
                    }
                    
                    // Prevent duplicate submissions
                    if (this.isSubmitting) {
                        return;
                    }
                    
                    // Check for duplicate downloads
                    const duplicateCheck = this.checkForDuplicate();
                    if (duplicateCheck.isDuplicate) {
                        this.statusMessage = { type: 'error', text: duplicateCheck.message };
                        return;
                    }
                    
                    this.isSubmitting = true;
                    this.statusMessage = null;
                    
                    try {
                        const response = await fetch('/api/downloads', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json'
                            },
                            body: JSON.stringify({
                                url: this.newDownload.url,
                                type: this.newDownload.type,
                                quality: this.newDownload.quality,
                                format: this.newDownload.format
                            })
                        });
                        
                        if (response.ok) {
                            this.statusMessage = { type: 'success', text: 'Download started successfully!' };
                            this.newDownload.url = '';
                            await this.loadDownloads();
                        } else {
                            const error = await response.text();
                            this.statusMessage = { type: 'error', text: 'Error: ' + error };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    }
                    
                    this.isSubmitting = false;
                    setTimeout(() => {
                        this.statusMessage = null;
                    }, 5000);
                },
                
                async deleteDownload(id) {
                    try {
                        const response = await fetch('/api/downloads/' + id, {
                            method: 'DELETE'
                        });
                        
                        if (response.ok) {
                            await this.loadDownloads();
                        } else {
                            this.statusMessage = { type: 'error', text: 'Failed to delete download' };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    }
                },
                
                async pauseDownload(id) {
                    try {
                        const response = await fetch('/api/downloads/' + id + '/pause', {
                            method: 'POST'
                        });
                        
                        if (response.ok) {
                            await this.loadDownloads();
                        } else {
                            this.statusMessage = { type: 'error', text: 'Failed to pause download' };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    }
                },
                
                async retryDownload(id) {
                    try {
                        const response = await fetch('/api/downloads/' + id + '/retry', {
                            method: 'POST'
                        });
                        
                        if (response.ok) {
                            await this.loadDownloads();
                            this.statusMessage = { type: 'success', text: 'Download restarted!' };
                        } else {
                            this.statusMessage = { type: 'error', text: 'Failed to retry download' };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    }
                },
                
                async clearAllQueued() {
                    if (!confirm('Are you sure you want to clear all queued downloads?')) {
                        return;
                    }
                    
                    try {
                        const response = await fetch('/api/downloads/clear-queued', {
                            method: 'POST'
                        });
                        
                        if (response.ok) {
                            await this.loadDownloads();
                            this.statusMessage = { type: 'success', text: 'All queued downloads cleared!' };
                        } else {
                            this.statusMessage = { type: 'error', text: 'Failed to clear queued downloads' };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    }
                },
                
                async deleteAllCompleted() {
                    if (!confirm('Are you sure you want to delete all completed downloads and their files? This action cannot be undone.')) {
                        return;
                    }
                    
                    try {
                        const response = await fetch('/api/downloads/delete-completed', {
                            method: 'POST'
                        });
                        
                        if (response.ok) {
                            await this.loadDownloads();
                            this.statusMessage = { type: 'success', text: 'All completed downloads and files deleted!' };
                        } else {
                            this.statusMessage = { type: 'error', text: 'Failed to delete completed downloads' };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    }
                },

                async clearAllFailed() {
                    if (!confirm('Are you sure you want to clear all failed downloads from the list?')) {
                        return;
                    }
                    
                    try {
                        const response = await fetch('/api/downloads/clear-failed', {
                            method: 'POST'
                        });
                        
                        if (response.ok) {
                            await this.loadDownloads();
                            this.statusMessage = { type: 'success', text: 'All failed downloads cleared!' };
                        } else {
                            this.statusMessage = { type: 'error', text: 'Failed to clear failed downloads' };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    }
                },
                
                async loadDownloads() {
                    try {
                        const response = await fetch('/api/downloads');
                        if (response.ok) {
                            const newDownloads = await response.json();
                            
                            // Only update if there are actual changes to prevent unnecessary re-renders
                            if (JSON.stringify(this.downloads) !== JSON.stringify(newDownloads)) {
                                this.downloads = newDownloads;
                            }
                            
                            this.isConnected = true;
                        }
                    } catch (error) {
                        this.isConnected = false;
                        console.error('Failed to load downloads:', error);
                    }
                },
                
                async loadSettings() {
                    try {
                        const response = await fetch('/api/config');
                        if (response.ok) {
                            this.settings = await response.json();
                        }
                    } catch (error) {
                        console.error('Failed to load settings:', error);
                    }
                },
                
                async saveSettings() {
                    this.isSavingSettings = true;
                    try {
                        const response = await fetch('/api/config', {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json'
                            },
                            body: JSON.stringify(this.settings)
                        });
                        
                        if (response.ok) {
                            this.statusMessage = { type: 'success', text: 'Settings saved successfully!' };
                            this.showSettings = false;
                        } else {
                            const error = await response.text();
                            this.statusMessage = { type: 'error', text: 'Failed to save settings: ' + error };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    } finally {
                        this.isSavingSettings = false;
                        setTimeout(() => {
                            this.statusMessage = null;
                        }, 5000);
                    }
                },
                
                async loadVersions() {
                    try {
                        const response = await fetch('/api/versions');
                        if (response.ok) {
                            this.versions = await response.json();
                        }
                    } catch (error) {
                        console.error('Failed to load versions:', error);
                    }
                },
                
                async checkForUpdates() {
                    this.isCheckingUpdates = true;
                    try {
                        const response = await fetch('/api/yt-dlp/version');
                        if (response.ok) {
                            this.updateInfo = await response.json();
                            
                            // Show notification if update is available and we're checking automatically
                            if (this.updateInfo.update_available && !this.showSettings) {
                                this.statusMessage = { 
                                    type: 'success', 
                                    text: 'yt-dlp update available! Current: ' + this.updateInfo.current_version + ', Latest: ' + this.updateInfo.latest_version + '. Check Settings to update.' 
                                };
                                setTimeout(() => {
                                    this.statusMessage = null;
                                }, 10000); // Show for 10 seconds
                            }
                        } else {
                            console.error('Failed to check for updates');
                        }
                    } catch (error) {
                        console.error('Failed to check for updates:', error);
                    } finally {
                        this.isCheckingUpdates = false;
                    }
                },
                
                async updateYtDlp() {
                    this.isUpdating = true;
                    try {
                        const response = await fetch('/api/yt-dlp/update', {
                            method: 'POST'
                        });
                        
                        if (response.ok) {
                            const result = await response.json();
                            this.statusMessage = { type: 'success', text: result.message || 'yt-dlp updated successfully!' };
                            
                            // Refresh update info and versions after successful update
                            await this.checkForUpdates();
                            await this.loadVersions();
                        } else {
                            const error = await response.text();
                            this.statusMessage = { type: 'error', text: 'Failed to update yt-dlp: ' + error };
                        }
                    } catch (error) {
                        this.statusMessage = { type: 'error', text: 'Network error: ' + error.message };
                    } finally {
                        this.isUpdating = false;
                        setTimeout(() => {
                            this.statusMessage = null;
                        }, 5000);
                    }
                }
            },
            
            mounted() {
                this.checkDarkMode();
                this.loadDownloads();
                this.loadSettings();
                this.loadVersions();
                
                // Check for yt-dlp updates on app load
                this.checkForUpdates();
                
                // Poll for updates every 2 seconds
                setInterval(() => {
                    this.loadDownloads();
                }, 2000);
            }
        }).mount('#app');
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
