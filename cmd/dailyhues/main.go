package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mgabor3141/dailyhues/internal/ai"
	"github.com/mgabor3141/dailyhues/internal/bing"
	"github.com/mgabor3141/dailyhues/internal/cache"
)

const (
	defaultCacheDir = "./cache_data"
	defaultLocale   = "en-US"
	defaultPort     = "8080"
	maxDaysBack     = 7
)

// Allowed locales for Bing wallpaper API (initialized in main from env or defaults)
var allowedLocales []string

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
	if os.Getenv("LOG_FORMAT") == "json" {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		slog.SetDefault(logger)
	}

	// Initialize allowed locales from environment or use defaults
	localesEnv := os.Getenv("ALLOWED_LOCALES")
	if localesEnv != "" {
		allowedLocales = strings.Split(localesEnv, ",")
		// Trim spaces from each locale
		for i := range allowedLocales {
			allowedLocales[i] = strings.TrimSpace(allowedLocales[i])
		}
		slog.Info("Using custom allowed locales from env", "locales", allowedLocales)
	} else {
		allowedLocales = []string{
			"en-US", "en-GB", "en-CA", "en-AU", "en-IN",
			"ja-JP", "zh-CN", "zh-TW", "de-DE", "fr-FR",
			"es-ES", "it-IT", "pt-BR", "ru-RU", "ko-KR",
		}
		slog.Info("Using default allowed locales", "locales", allowedLocales)
	}

	// Get cache directory from environment or use default
	cacheDataDir := os.Getenv("CACHE_DIR")
	if cacheDataDir == "" {
		cacheDataDir = defaultCacheDir
	}
	slog.Info("Using cache directory", "dir", cacheDataDir)

	// Get OpenRouter API key from environment
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		slog.Error("OPENROUTER_API_KEY environment variable is required")
	}

	// Initialize caches
	requestCache, err := cache.NewRequestCache(cacheDataDir)
	if err != nil {
		slog.Error("Failed to initialize request cache", "error", err)
	}

	analysisCache, err := cache.NewAnalysisCache(cacheDataDir)
	if err != nil {
		slog.Error("Failed to initialize analysis cache", "error", err)
	}

	// Load all existing cache files into memory on startup
	if err := requestCache.LoadAll(); err != nil {
		slog.Error("Failed to load request cache", "error", err)
	}
	if err := analysisCache.LoadAll(); err != nil {
		slog.Error("Failed to load analysis cache", "error", err)
	}

	// Initialize app
	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
		bingClient:    bing.NewClient(defaultLocale),
		aiAnalyzer:    ai.NewAnalyzer(apiKey),
	}

	// Set up routes
	http.HandleFunc("/", handleLandingPage)
	http.HandleFunc("/api/colors", app.handleGetColors)
	http.HandleFunc("/health", handleHealth)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	slog.Info(fmt.Sprintf(`

dailyhues starting on port %s
Endpoints:
    GET /
    GET /api/colors?locale=%s&daysAgo=0
    GET /health

`, port, defaultLocale))

	server := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}

// handleLandingPage returns a simple HTML landing page
func handleLandingPage(w http.ResponseWriter, r *http.Request) {
	// Only handle root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>dailyhues - Bing Wallpaper Color Palette API</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            line-height: 1.6;
            max-width: 800px;
            margin: 0 auto;
            padding: 2rem;
            color: #e6e6e6;
            background: #0d1117;
        }
        h1 { color: #58a6ff; margin-bottom: 0.5rem; }
        .subtitle { color: #8b949e; margin-top: 0; }
        a { color: #58a6ff; text-decoration: none; }
        a:hover { text-decoration: underline; }
        code {
            background: #161b22;
            padding: 0.2rem 0.4rem;
            border-radius: 3px;
            font-family: 'Courier New', monospace;
            color: #e6e6e6;
        }
        .code-block {
            position: relative;
            margin: 2rem 0;
        }
        pre {
            background: #161b22;
            border: 1px solid #30363d;
            border-radius: 6px;
            padding: 1rem;
            overflow-x: auto;
            margin: 0;
        }
        pre code {
            background: none;
            padding: 0;
            color: #c9d1d9;
            font-size: 0.9rem;
        }
        .copy-btn {
            position: absolute;
            top: 0.5rem;
            right: 0.5rem;
            background: #21262d;
            border: 1px solid #30363d;
            color: #c9d1d9;
            padding: 0.4rem 0.8rem;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.85rem;
            transition: background 0.2s;
        }
        .copy-btn:hover {
            background: #30363d;
        }
        .copy-btn.copied {
            color: #3fb950;
        }
        .links { margin-top: 2rem; }
    </style>
</text>
</head>
<body>
    <h1>dailyhues</h1>
    <p class="subtitle">AI-extracted color palettes from Bing's daily wallpaper</p>

    <div class="code-block">
        <button class="copy-btn" onclick="copyCode()">Copy</button>
        <pre><code>curl https://dailyhues.mgabor.hu/api/colors</code></pre>
    </div>

    <div class="links">
        <p><a href="https://github.com/mgabor3141/dailyhues">View on GitHub</a> for full documentation and examples</p>
    </div>

    <script>
        function copyCode() {
            const code = document.querySelector('pre code').textContent;
            navigator.clipboard.writeText(code).then(() => {
                const btn = document.querySelector('.copy-btn');
                btn.textContent = 'Copied!';
                btn.classList.add('copied');
                setTimeout(() => {
                    btn.textContent = 'Copy';
                    btn.classList.remove('copied');
                }, 2000);
            });
        }
    </script>
</body>
</html>`
	fmt.Fprint(w, html)
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
		slog.Info("Failed to download wallpaper", "error", err)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to download wallpaper: %v", err))
		return
	}

	slog.Info("Downloaded wallpaper", "title", info.Title, "bytes", len(imageData))

	// Step 3: Generate image hash (this is our unique identifier)
	imageHash := cache.HashImage(imageData)
	slog.Info("Image hash", "hash", imageHash)

	// Step 4: Check analysis cache by image hash
	if analysisEntry := app.analysisCache.Get(imageHash); analysisEntry != nil {
		// Analysis exists! Just cache the request metadata and return
		slog.Info("Analysis cache hit for image hash", "hash", imageHash)

		expiresAt := getNextHourBoundary()
		if err := app.requestCache.Set(locale, daysAgo, imageHash, info.ImageURLs, info.Title, info.Copyright, info.CopyrightLink, info.StartDate, info.FullStartDate, info.EndDate, expiresAt); err != nil {
			slog.Info("Failed to cache request", "error", err)
		}

		response := buildColorThemeFromInfo(info, analysisEntry)
		respondWithJSON(w, http.StatusOK, response)
		return
	}

	// Step 5: Acquire mutex for this image hash (prevents duplicate analysis)
	imageMutex := app.analysisCache.GetMutex(imageHash)
	imageMutex.Lock()
	defer imageMutex.Unlock()
	defer app.analysisCache.ReleaseMutex(imageHash)

	// Step 6: Double-check analysis cache (another goroutine might have completed)
	if analysisEntry := app.analysisCache.Get(imageHash); analysisEntry != nil {
		slog.Info("Analysis completed by another request for image hash", "hash", imageHash)

		expiresAt := getNextHourBoundary()
		if err := app.requestCache.Set(locale, daysAgo, imageHash, info.ImageURLs, info.Title, info.Copyright, info.CopyrightLink, info.StartDate, info.FullStartDate, info.EndDate, expiresAt); err != nil {
			slog.Info("Failed to cache request", "error", err)
		}

		response := buildColorThemeFromInfo(info, analysisEntry)
		respondWithJSON(w, http.StatusOK, response)
		return
	}

	// Step 7: Analyze colors with AI (image already downloaded)
	slog.Info("Starting AI analysis for image hash", "hash", imageHash)
	colors, err := app.aiAnalyzer.AnalyzeColors(imageData, imageHash, info.Title, info.Copyright)
	if err != nil {
		slog.Info("Failed to analyze colors", "error", err)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to analyze colors: %v", err))
		return
	}

	slog.Info("Extracted colors for image hash", "hash", imageHash, "colors", colors)

	// Step 8: Store analysis in cache (shared across all locales with this image)
	if err := app.analysisCache.Set(imageHash, colors); err != nil {
		slog.Info("Failed to cache analysis", "error", err)
	}

	// Step 9: Store request metadata in cache
	expiresAt := getNextHourBoundary()
	if err := app.requestCache.Set(locale, daysAgo, imageHash, info.ImageURLs, info.Title, info.Copyright, info.CopyrightLink, info.StartDate, info.FullStartDate, info.EndDate, expiresAt); err != nil {
		slog.Info("Failed to cache request", "error", err)
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
		return 0, fmt.Errorf("invalid daysAgo parameter. Must be an integer")
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

	return "", fmt.Errorf("invalid locale. Supported locales: %s", strings.Join(allowedLocales, ", "))
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
