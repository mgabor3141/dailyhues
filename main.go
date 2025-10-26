package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mgabor3141/wallpaper-highlight/ai"
	"github.com/mgabor3141/wallpaper-highlight/bing"
	"github.com/mgabor3141/wallpaper-highlight/cache"
)

const (
	cacheDataDir  = "./cache_data"
	defaultLocale = "en-US"
	defaultPort   = "8080"
	maxDaysBack   = 7
)

// Allowed locales for Bing wallpaper API
var allowedLocales = []string{
	"en-US", "en-GB", "en-CA", "en-AU", "en-IN",
	"ja-JP", "zh-CN", "zh-TW", "de-DE", "fr-FR",
	"es-ES", "it-IT", "pt-BR", "ru-RU", "ko-KR",
}

// ColorTheme represents the response with extracted colors from a wallpaper
type ColorTheme struct {
	StartDate     string                 `json:"startdate"`
	FullStartDate string                 `json:"fullstartdate"`
	EndDate       string                 `json:"enddate"`
	Images        map[string]string      `json:"images"`
	Colors        map[string]interface{} `json:"colors"`
	Title         string                 `json:"title"`
	Copyright     string                 `json:"copyright"`
	CopyrightLink string                 `json:"copyright_link"`
	CachedAt      string                 `json:"cached_at"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error string `json:"error"`
}

// App holds the application dependencies
type App struct {
	requestCache  *cache.RequestCache
	analysisCache *cache.AnalysisCache
	bingClient    *bing.Client
	aiAnalyzer    *ai.Analyzer
}

func main() {
	// Get OpenRouter API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY environment variable is required")
	}

	// Initialize caches
	requestCache, err := cache.NewRequestCache(cacheDataDir)
	if err != nil {
		log.Fatalf("Failed to initialize request cache: %v", err)
	}

	analysisCache, err := cache.NewAnalysisCache(cacheDataDir)
	if err != nil {
		log.Fatalf("Failed to initialize analysis cache: %v", err)
	}

	// Load all existing cache files into memory on startup
	if err := requestCache.LoadAll(); err != nil {
		log.Printf("Warning: Failed to load request cache: %v", err)
	}
	if err := analysisCache.LoadAll(); err != nil {
		log.Printf("Warning: Failed to load analysis cache: %v", err)
	}

	// Initialize app
	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
		bingClient:    bing.NewClient(defaultLocale),
		aiAnalyzer:    ai.NewAnalyzer(apiKey),
	}

	// Set up routes
	http.HandleFunc("/api/colors", app.handleGetColors)
	http.HandleFunc("/health", handleHealth)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	fmt.Printf("🎨 Wallpaper Color Analysis API starting on port %s\n", port)
	fmt.Printf("📍 Endpoints:\n")
	fmt.Printf("   GET /api/colors?daysAgo=0&locale=%s\n", defaultLocale)
	fmt.Printf("   GET /health\n\n")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// handleHealth returns a simple health check response
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// handleGetColors is the main endpoint for getting wallpaper colors
func (app *App) handleGetColors(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Validate and parse daysAgo parameter
	daysAgo, err := validateDaysAgo(r.URL.Query().Get("daysAgo"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate locale parameter
	locale, err := validateLocale(r.URL.Query().Get("locale"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Step 1: Check request cache (with TTL validation)
	if reqEntry := app.requestCache.Get(locale, daysAgo); reqEntry != nil {
		// Check if cache is still valid (not past the expiration time)
		if time.Now().Before(reqEntry.ExpiresAt) {
			// Request cached, now check if we have the analysis
			if analysisEntry := app.analysisCache.Get(reqEntry.ImageHash); analysisEntry != nil {
				response := buildColorTheme(reqEntry, analysisEntry)
				respondWithJSON(w, http.StatusOK, response)
				return
			}
		}
	}

	// Step 2: Download wallpaper metadata and image from Bing
	app.bingClient.SetLocale(locale)
	imageData, info, err := app.bingClient.GetWallpaperByDaysAgo(daysAgo)
	if err != nil {
		log.Printf("Failed to download wallpaper: %v", err)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to download wallpaper: %v", err))
		return
	}

	log.Printf("Downloaded wallpaper: %s (%d bytes)", info.Title, len(imageData))

	// Step 3: Generate image hash (this is our unique identifier)
	imageHash := cache.HashImage(imageData)
	log.Printf("Image hash: %s", imageHash)

	// Step 4: Check analysis cache by image hash
	if analysisEntry := app.analysisCache.Get(imageHash); analysisEntry != nil {
		// Analysis exists! Just cache the request metadata and return
		log.Printf("Analysis cache hit for image hash: %s", imageHash)

		expiresAt := getNextHourBoundary()
		if err := app.requestCache.Set(locale, daysAgo, imageHash, info.ImageURLs, info.Title, info.Copyright, info.CopyrightLink, info.StartDate, info.FullStartDate, info.EndDate, expiresAt); err != nil {
			log.Printf("Failed to cache request: %v", err)
		}

		response := buildColorThemeFromInfo(info, analysisEntry)
		respondWithJSON(w, http.StatusOK, response)
		return
	}

	// Step 5: Acquire mutex for this image hash (prevents duplicate analysis)
	imageMutex := app.analysisCache.GetMutex(imageHash)
	imageMutex.Lock()
	defer imageMutex.Unlock()

	// Step 6: Double-check analysis cache (another goroutine might have completed)
	if analysisEntry := app.analysisCache.Get(imageHash); analysisEntry != nil {
		log.Printf("Analysis completed by another request for image hash: %s", imageHash)

		expiresAt := getNextHourBoundary()
		if err := app.requestCache.Set(locale, daysAgo, imageHash, info.ImageURLs, info.Title, info.Copyright, info.CopyrightLink, info.StartDate, info.FullStartDate, info.EndDate, expiresAt); err != nil {
			log.Printf("Failed to cache request: %v", err)
		}

		response := buildColorThemeFromInfo(info, analysisEntry)
		respondWithJSON(w, http.StatusOK, response)
		return
	}

	// Step 7: Analyze colors with AI (image already downloaded)
	log.Printf("Starting AI analysis for image hash: %s", imageHash)
	colors, err := app.aiAnalyzer.AnalyzeColors(imageData, imageHash, info.Title, info.Copyright)
	if err != nil {
		log.Printf("Failed to analyze colors: %v", err)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to analyze colors: %v", err))
		return
	}

	log.Printf("Extracted colors for image hash %s: %v", imageHash, colors)

	// Step 8: Store analysis in cache (shared across all locales with this image)
	if err := app.analysisCache.Set(imageHash, colors); err != nil {
		log.Printf("Failed to cache analysis: %v", err)
	}

	// Step 9: Store request metadata in cache
	expiresAt := getNextHourBoundary()
	if err := app.requestCache.Set(locale, daysAgo, imageHash, info.ImageURLs, info.Title, info.Copyright, info.CopyrightLink, info.StartDate, info.FullStartDate, info.EndDate, expiresAt); err != nil {
		log.Printf("Failed to cache request: %v", err)
	}

	// Step 10: Return response
	response := ColorTheme{
		StartDate:     info.StartDate,
		FullStartDate: info.FullStartDate,
		EndDate:       info.EndDate,
		Images:        info.ImageURLs,
		Colors:        colors,
		Title:         info.Title,
		Copyright:     info.Copyright,
		CopyrightLink: info.CopyrightLink,
		CachedAt:      time.Now().Format(time.RFC3339),
	}

	respondWithJSON(w, http.StatusOK, response)
}

// validateDaysAgo validates the daysAgo parameter
func validateDaysAgo(daysAgoParam string) (int, error) {
	// Default to today (0 days ago) if not provided
	if daysAgoParam == "" {
		return 0, nil
	}

	// Parse as integer
	var daysAgo int
	_, err := fmt.Sscanf(daysAgoParam, "%d", &daysAgo)
	if err != nil {
		return 0, fmt.Errorf("Invalid daysAgo parameter. Must be an integer")
	}

	// Validate range
	if daysAgo < 0 {
		return 0, fmt.Errorf("daysAgo cannot be negative")
	}

	if daysAgo > maxDaysBack {
		return 0, fmt.Errorf("daysAgo too large. Bing only keeps wallpapers for the last %d days", maxDaysBack)
	}

	return daysAgo, nil
}

// validateLocale validates the locale parameter
func validateLocale(locale string) (string, error) {
	// Default to en-US if not provided
	if locale == "" {
		return defaultLocale, nil
	}

	// Check if locale is allowed
	for _, allowed := range allowedLocales {
		if locale == allowed {
			return locale, nil
		}
	}

	return "", fmt.Errorf("Invalid locale. Supported locales: %s", strings.Join(allowedLocales, ", "))
}

// buildColorTheme creates a ColorTheme response from cache entries
func buildColorTheme(reqEntry *cache.RequestEntry, analysisEntry *cache.AnalysisEntry) ColorTheme {
	return ColorTheme{
		StartDate:     reqEntry.StartDate,
		FullStartDate: reqEntry.FullStartDate,
		EndDate:       reqEntry.EndDate,
		Images:        reqEntry.ImageURLs,
		Colors:        analysisEntry.Colors,
		Title:         reqEntry.Title,
		Copyright:     reqEntry.Copyright,
		CopyrightLink: reqEntry.CopyrightLink,
		CachedAt:      time.Now().Format(time.RFC3339),
	}
}

// buildColorThemeFromInfo creates a ColorTheme response from wallpaper info and analysis
func buildColorThemeFromInfo(info *bing.WallpaperInfo, analysisEntry *cache.AnalysisEntry) ColorTheme {
	return ColorTheme{
		StartDate:     info.StartDate,
		FullStartDate: info.FullStartDate,
		EndDate:       info.EndDate,
		Images:        info.ImageURLs,
		Colors:        analysisEntry.Colors,
		Title:         info.Title,
		Copyright:     info.Copyright,
		CopyrightLink: info.CopyrightLink,
		CachedAt:      time.Now().Format(time.RFC3339),
	}
}

// getNextHourBoundary returns the time at the start of the next hour
func getNextHourBoundary() time.Time {
	now := time.Now()
	return now.Truncate(time.Hour).Add(time.Hour)
}

// respondWithJSON is a helper to send JSON responses
func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// respondWithError is a helper to send error responses
func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	respondWithJSON(w, statusCode, ErrorResponse{Error: message})
}
