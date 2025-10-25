package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

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

// getCacheKey generates a cache key from date and locale
func getCacheKey(date, locale string) string {
	return date + "_" + locale
}

// ColorTheme represents the response with extracted colors from a wallpaper
type ColorTheme struct {
	Date      string            `json:"date"`
	Images    map[string]string `json:"images"`
	Colors    map[string]string `json:"colors"`
	CachedAt  string            `json:"cached_at"`
	FromCache bool              `json:"from_cache"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error string `json:"error"`
}

// App holds the application dependencies
type App struct {
	cache       *cache.Cache
	bingClient  *bing.Client
	aiAnalyzer  *ai.Analyzer
	processMu   sync.Mutex // Ensures only one analysis runs at a time per date
	processing  map[string]*sync.Mutex
	processingL sync.Mutex
}

func main() {
	// Get OpenRouter API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENROUTER_API_KEY environment variable is required")
	}

	// Initialize cache and load existing cache files
	cacheInstance, err := cache.New("./cache_data")
	if err != nil {
		log.Fatalf("Failed to initialize cache: %v", err)
	}

	// Load all existing cache files into memory on startup
	if err := cacheInstance.LoadAll(); err != nil {
		log.Printf("Warning: Failed to load cache files: %v", err)
	}

	// Initialize app
	app := &App{
		cache:      cacheInstance,
		bingClient: bing.NewClient("en-US"),
		aiAnalyzer: ai.NewAnalyzer(apiKey),
		processing: make(map[string]*sync.Mutex),
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

	// Generate cache key from date and locale
	cacheKey := getCacheKey(dateParam, locale)

	// Check cache first
	cached, err := app.cache.Get(cacheKey)
	if err != nil {
		log.Printf("Cache read error: %v", err)
	}

	if cached != nil {
		// Cache hit! Return immediately
		response := ColorTheme{
			Date:      cached.Date,
			Images:    cached.Images,
			Colors:    cached.Colors,
			CachedAt:  cached.CachedAt.Format(time.RFC3339),
			FromCache: true,
		}
		respondWithJSON(w, http.StatusOK, response)
		return
	}

	// Cache miss - need to process
	// Get or create a mutex for this specific date+locale to prevent duplicate processing
	dateMutex := app.getDateMutex(cacheKey)
	dateMutex.Lock()
	defer dateMutex.Unlock()

	// Double-check cache after acquiring lock (another request might have completed)
	cached, err = app.cache.Get(cacheKey)
	if err != nil {
		log.Printf("Cache read error: %v", err)
	}

	if cached != nil {
		response := ColorTheme{
			Date:      cached.Date,
			Images:    cached.Images,
			Colors:    cached.Colors,
			CachedAt:  cached.CachedAt.Format(time.RFC3339),
			FromCache: true,
		}
		respondWithJSON(w, http.StatusOK, response)
		return
	}

	// Still not in cache, do the actual work
	log.Printf("Processing wallpaper for date: %s", dateParam)

	// Step 1: Download wallpaper from Bing
	app.bingClient.SetLocale(locale)
	imageData, info, err := app.bingClient.GetWallpaper(dateParam)
	if err != nil {
		log.Printf("Failed to download wallpaper: %v", err)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to download wallpaper: %v", err))
		return
	}

	log.Printf("Downloaded wallpaper: %s (%d bytes)", info.Title, len(imageData))

	// Step 2: Analyze colors with AI
	colors, err := app.aiAnalyzer.AnalyzeColors(imageData)
	if err != nil {
		log.Printf("Failed to analyze colors: %v", err)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to analyze colors: %v", err))
		return
	}

	log.Printf("Extracted colors: %v", colors)

	// Step 3: Store in cache
	if err := app.cache.Set(cacheKey, info.ImageURLs, colors); err != nil {
		log.Printf("Failed to cache result: %v", err)
		// Don't fail the request if caching fails
	}

	// Step 4: Return response
	response := ColorTheme{
		Date:      dateParam,
		Images:    info.ImageURLs,
		Colors:    colors,
		CachedAt:  time.Now().Format(time.RFC3339),
		FromCache: false,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// getDateMutex gets or creates a mutex for a specific cache key (date + locale)
// This ensures only one goroutine processes a given date+locale combination at a time
func (app *App) getDateMutex(cacheKey string) *sync.Mutex {
	app.processingL.Lock()
	defer app.processingL.Unlock()

	if mu, exists := app.processing[cacheKey]; exists {
		return mu
	}

	mu := &sync.Mutex{}
	app.processing[cacheKey] = mu
	return mu
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
