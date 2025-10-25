package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/mgabor3141/wallpaper-highlight/ai"
	"github.com/mgabor3141/wallpaper-highlight/bing"
	"github.com/mgabor3141/wallpaper-highlight/cache"
)

// Allowed locales for Bing wallpaper API
var allowedLocales = map[string]bool{
	"en-US": true,
	"en-GB": true,
	"en-CA": true,
	"en-AU": true,
	"en-IN": true,
	"ja-JP": true,
	"zh-CN": true,
	"zh-TW": true,
	"de-DE": true,
	"fr-FR": true,
	"es-ES": true,
	"it-IT": true,
	"pt-BR": true,
	"ru-RU": true,
	"ko-KR": true,
}

// ColorTheme represents the response with extracted colors from a wallpaper
type ColorTheme struct {
	Date          string            `json:"date"`
	Images        map[string]string `json:"images"`
	Colors        map[string]string `json:"colors"`
	Copyright     string            `json:"copyright"`
	CopyrightLink string            `json:"copyright_link"`
	CachedAt      string            `json:"cached_at"`
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
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	// Get OpenRouter API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY environment variable is required")
	}

	// Initialize caches
	requestCache, err := cache.NewRequestCache("./cache_data")
	if err != nil {
		log.Fatalf("Failed to initialize request cache: %v", err)
	}

	analysisCache, err := cache.NewAnalysisCache("./cache_data")
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
		bingClient:    bing.NewClient("en-US"),
		aiAnalyzer:    ai.NewAnalyzer(apiKey),
	}

	// Set up routes
	http.HandleFunc("/api/colors", app.handleGetColors)
	http.HandleFunc("/health", handleHealth)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🎨 Wallpaper Color Analysis API starting on port %s\n", port)
	fmt.Printf("📍 Endpoints:\n")
	fmt.Printf("   GET /api/colors?date=YYYY-MM-DD\n")
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

	// Get date parameter (defaults to today if not provided)
	dateParam := r.URL.Query().Get("date")
	if dateParam == "" {
		dateParam = time.Now().Format("2006-01-02")
	}

	// Validate date format
	targetDate, err := time.Parse("2006-01-02", dateParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
		return
	}

	// Validate date is not in the future
	today := time.Now().Truncate(24 * time.Hour)
	if targetDate.After(today) {
		respondWithError(w, http.StatusBadRequest, "Cannot fetch wallpaper for future dates")
		return
	}

	// Validate date is within last 7 days (Bing's API limitation)
	daysAgo := int(today.Sub(targetDate).Hours() / 24)
	if daysAgo > 7 {
		respondWithError(w, http.StatusBadRequest, "Date too old. Bing only keeps wallpapers for the last 7 days")
		return
	}

	// Get locale parameter (defaults to en-US if not provided)
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = "en-US"
	}

	// Validate locale
	if !allowedLocales[locale] {
		respondWithError(w, http.StatusBadRequest, "Invalid locale. Supported locales: en-US, en-GB, en-CA, en-AU, en-IN, ja-JP, zh-CN, zh-TW, de-DE, fr-FR, es-ES, it-IT, pt-BR, ru-RU, ko-KR")
		return
	}

	// Step 1: Check request cache
	if reqEntry := app.requestCache.Get(dateParam, locale); reqEntry != nil {
		// Request cached, now check if we have the analysis
		if analysisEntry := app.analysisCache.Get(reqEntry.ImageHash); analysisEntry != nil {
			response := ColorTheme{
				Date:          dateParam,
				Images:        reqEntry.ImageURLs,
				Colors:        analysisEntry.Colors,
				Copyright:     reqEntry.Copyright,
				CopyrightLink: reqEntry.CopyrightLink,
				CachedAt:      time.Now().Format(time.RFC3339),
			}
			respondWithJSON(w, http.StatusOK, response)
			return
		}
	}

	// Step 2: Download wallpaper metadata and image from Bing
	app.bingClient.SetLocale(locale)
	imageData, info, err := app.bingClient.GetWallpaper(dateParam)
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

		if err := app.requestCache.Set(dateParam, locale, imageHash, info.ImageURLs, info.Copyright, info.CopyrightLink); err != nil {
			log.Printf("Failed to cache request: %v", err)
		}

		response := ColorTheme{
			Date:          dateParam,
			Images:        info.ImageURLs,
			Colors:        analysisEntry.Colors,
			Copyright:     info.Copyright,
			CopyrightLink: info.CopyrightLink,
			CachedAt:      time.Now().Format(time.RFC3339),
		}
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

		if err := app.requestCache.Set(dateParam, locale, imageHash, info.ImageURLs, info.Copyright, info.CopyrightLink); err != nil {
			log.Printf("Failed to cache request: %v", err)
		}

		response := ColorTheme{
			Date:          dateParam,
			Images:        info.ImageURLs,
			Colors:        analysisEntry.Colors,
			Copyright:     info.Copyright,
			CopyrightLink: info.CopyrightLink,
			CachedAt:      time.Now().Format(time.RFC3339),
		}
		respondWithJSON(w, http.StatusOK, response)
		return
	}

	// Step 7: Analyze colors with AI (image already downloaded)
	log.Printf("Starting AI analysis for image hash: %s", imageHash)
	colors, err := app.aiAnalyzer.AnalyzeColors(imageData)
	if err != nil {
		log.Printf("Failed to download wallpaper: %v", err)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to download wallpaper: %v", err))
		return
	}

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
	if err := app.requestCache.Set(dateParam, locale, imageHash, info.ImageURLs, info.Copyright, info.CopyrightLink); err != nil {
		log.Printf("Failed to cache request: %v", err)
	}

	// Step 10: Return response
	response := ColorTheme{
		Date:          dateParam,
		Images:        info.ImageURLs,
		Colors:        colors,
		Copyright:     info.Copyright,
		CopyrightLink: info.CopyrightLink,
		CachedAt:      time.Now().Format(time.RFC3339),
	}

	respondWithJSON(w, http.StatusOK, response)
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
