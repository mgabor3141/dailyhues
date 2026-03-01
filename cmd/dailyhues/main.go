package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mgabor3141/dailyhues/internal/ai"
	"github.com/mgabor3141/dailyhues/internal/bing"
	"github.com/mgabor3141/dailyhues/internal/cache"
)

const (
	defaultCacheDir = "./cache_data"
	defaultPort     = "8080"
	maxDaysBack     = 7
)

// ColorTheme is the API response for both v1 and v2
type ColorTheme struct {
	StartDate     string                 `json:"startdate"`
	FullStartDate string                 `json:"fullstartdate"`
	EndDate       string                 `json:"enddate"`
	Images        map[string]string      `json:"images"`
	Colors        map[string]interface{} `json:"colors"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description,omitempty"`
	Copyright     string                 `json:"copyright"`
	CopyrightLink string                 `json:"copyright_link"`
	CachedAt      string                 `json:"cached_at"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error string `json:"error"`
}

// wallpaperResult is the internal result from ensureResult
type wallpaperResult struct {
	startDate     string
	fullStartDate string
	endDate       string
	imageURLs     map[string]string
	imageHash     string
	colors        map[string]interface{}
	bingTitle     string // original Bing title
	copyright     string
	copyrightLink string
	title         string // translated title (or bingTitle if no translation)
	description   string // translated description (empty if no translation)
	enUSMatch     bool   // whether en-US market had this same image
}

// App holds the application dependencies and instance configuration
type App struct {
	requestCache     *cache.RequestCache
	analysisCache    *cache.AnalysisCache
	translationCache *cache.TranslationCache
	bingClient       *bing.Client
	aiAnalyzer       *ai.Analyzer

	// Instance configuration — controls what v2 accepts
	allowedRegions  []string // e.g., ["global", "en-US"]; first is the default
	allowedLanguages []string // e.g., ["English"]; first is default; empty = no translation
	defaultRegion   string
	defaultLanguage string
}

func main() {
	if os.Getenv("LOG_FORMAT") == "json" {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		slog.SetDefault(logger)
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
		os.Exit(1)
	}

	// Parse instance configuration
	allowedRegions := parseCSVEnv("ALLOWED_REGIONS", "global")
	if len(allowedRegions) == 0 {
		slog.Error("ALLOWED_REGIONS must contain at least one value")
		os.Exit(1)
	}
	allowedLanguages := parseCSVEnv("ALLOWED_LANGUAGES", "English")

	slog.Info("Instance configuration",
		"allowedRegions", allowedRegions,
		"allowedLanguages", allowedLanguages,
	)

	// Initialize caches
	requestCache, err := cache.NewRequestCache(cacheDataDir)
	if err != nil {
		slog.Error("Failed to initialize request cache", "error", err)
		os.Exit(1)
	}

	analysisCache, err := cache.NewAnalysisCache(cacheDataDir)
	if err != nil {
		slog.Error("Failed to initialize analysis cache", "error", err)
		os.Exit(1)
	}

	translationCache, err := cache.NewTranslationCache(cacheDataDir)
	if err != nil {
		slog.Error("Failed to initialize translation cache", "error", err)
		os.Exit(1)
	}

	// Load all existing cache files into memory on startup
	if err := requestCache.LoadAll(); err != nil {
		slog.Error("Failed to load request cache", "error", err)
		os.Exit(1)
	}
	if err := analysisCache.LoadAll(); err != nil {
		slog.Error("Failed to load analysis cache", "error", err)
		os.Exit(1)
	}
	if err := translationCache.LoadAll(); err != nil {
		slog.Error("Failed to load translation cache", "error", err)
		os.Exit(1)
	}

	// Determine defaults
	defaultLanguage := ""
	if len(allowedLanguages) > 0 {
		defaultLanguage = allowedLanguages[0]
	}

	// Initialize app
	app := &App{
		requestCache:     requestCache,
		analysisCache:    analysisCache,
		translationCache: translationCache,
		bingClient:       bing.NewClient("en-US"),
		aiAnalyzer:       ai.NewAnalyzer(apiKey),
		allowedRegions:   allowedRegions,
		allowedLanguages: allowedLanguages,
		defaultRegion:    allowedRegions[0],
		defaultLanguage:  defaultLanguage,
	}

	// Set up routes
	http.HandleFunc("/", handleLandingPage)
	http.HandleFunc("/api/colors", app.handleGetColors)
	http.HandleFunc("/api/v2/colors", app.handleGetColorsV2)
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
    GET /api/colors?daysAgo=0                                  (v1, deprecated)
    GET /api/v2/colors?region=%s&language=%s&daysAgo=0    (v2)
    GET /health

`, port, app.defaultRegion, app.defaultLanguage))

	server := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server failed to start", "error", err)
	}
}

// =============================================================================
// v1 handler — deprecated, thin wrapper over v2 defaults
// =============================================================================

func (app *App) handleGetColors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	daysAgo, err := validateDaysAgo(r.URL.Query().Get("daysAgo"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := app.ensureResult(daysAgo, app.defaultRegion, app.defaultLanguage)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get wallpaper: %v", err))
		return
	}

	resp := ColorTheme{
		StartDate:     result.startDate,
		FullStartDate: result.fullStartDate,
		EndDate:       result.endDate,
		Images:        result.imageURLs,
		Colors:        result.colors,
		Copyright:     result.copyright,
		CopyrightLink: result.copyrightLink,
		CachedAt:      time.Now().Format(time.RFC3339),
	}

	if result.enUSMatch {
		// en-US has the same image — return old v1 format
		resp.Title = result.bingTitle
	} else {
		// Image differs from en-US — serve it anyway but signal deprecation
		resp.Title = result.title
		resp.Description = result.description
		resp.Copyright += " | ⚠ This endpoint is deprecated. Please use /api/v2/colors"
	}

	respondWithJSON(w, http.StatusOK, resp)
}

// =============================================================================
// v2 handler — gated by instance config
// =============================================================================

func (app *App) handleGetColorsV2(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	daysAgo, err := validateDaysAgo(r.URL.Query().Get("daysAgo"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	region, err := app.validateRegion(r.URL.Query().Get("region"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	language, err := app.validateLanguage(r.URL.Query().Get("language"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := app.ensureResult(daysAgo, region, language)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get wallpaper: %v", err))
		return
	}

	resp := ColorTheme{
		StartDate:     result.startDate,
		FullStartDate: result.fullStartDate,
		EndDate:       result.endDate,
		Images:        result.imageURLs,
		Colors:        result.colors,
		Title:         result.title,
		Description:   result.description,
		Copyright:     result.copyright,
		CopyrightLink: result.copyrightLink,
		CachedAt:      time.Now().Format(time.RFC3339),
	}

	respondWithJSON(w, http.StatusOK, resp)
}

// =============================================================================
// Core logic — shared between v1 and v2
// =============================================================================

// ensureResult fetches (or cache-hits) the wallpaper, colors, and optional
// translation for the given region + language. This is the single code path
// that both v1 and v2 use.
func (app *App) ensureResult(daysAgo int, region, language string) (*wallpaperResult, error) {
	cacheKey := region

	// --- Fast path: request cache hit + analysis cache hit ---
	if reqEntry := app.requestCache.Get(cacheKey, daysAgo); reqEntry != nil {
		if time.Now().Before(reqEntry.ExpiresAt) {
			if analysisEntry := app.analysisCache.Get(reqEntry.ImageHash); analysisEntry != nil {
				slog.Info("Full cache hit", "region", region, "daysAgo", daysAgo)

				result := &wallpaperResult{
					startDate:     reqEntry.StartDate,
					fullStartDate: reqEntry.FullStartDate,
					endDate:       reqEntry.EndDate,
					imageURLs:     reqEntry.ImageURLs,
					imageHash:     reqEntry.ImageHash,
					colors:        analysisEntry.Colors,
					bingTitle:     reqEntry.Title,
					copyright:     reqEntry.Copyright,
					copyrightLink: reqEntry.CopyrightLink,
					title:         reqEntry.Title, // default to Bing title
					enUSMatch:     reqEntry.EnUSMatch,
				}

				// Attach translation if language requested
				if language != "" {
					title, desc, err := app.ensureTranslation(
						reqEntry.ImageHash, reqEntry.Title, reqEntry.Copyright, language,
					)
					if err != nil {
						slog.Info("Translation failed on cache path", "error", err)
					} else {
						result.title = title
						result.description = desc
					}
				}

				return result, nil
			}
		}
	}

	// --- Slow path: download wallpaper + analyze ---
	var imageData []byte
	var info *bing.WallpaperInfo
	var enUSMatch bool

	if region == "global" {
		slog.Info("Fetching global wallpaper", "daysAgo", daysAgo)
		var matchingMarkets map[string]bool
		var err error
		imageData, info, matchingMarkets, err = app.bingClient.FindMostCommonWallpaper(daysAgo, bing.AllMarkets, "en-US")
		if err != nil {
			return nil, fmt.Errorf("global wallpaper fetch: %w", err)
		}
		enUSMatch = matchingMarkets["en-US"]
	} else {
		slog.Info("Fetching wallpaper for region", "region", region, "daysAgo", daysAgo)
		client := bing.NewClient(region)
		var err error
		imageData, info, err = client.GetWallpaperByDaysAgo(daysAgo)
		if err != nil {
			return nil, fmt.Errorf("wallpaper fetch for %s: %w", region, err)
		}
		enUSMatch = (region == "en-US")
	}

	slog.Info("Downloaded wallpaper", "title", info.Title, "bytes", len(imageData))

	// Color analysis (shared by image hash)
	imageHash := cache.HashImage(imageData)
	colors, err := app.ensureColorAnalysis(imageData, imageHash, info)
	if err != nil {
		return nil, fmt.Errorf("color analysis: %w", err)
	}

	// Cache request metadata
	if err := app.requestCache.Set(&cache.RequestEntry{
		Region:        cacheKey,
		DaysAgo:       daysAgo,
		ImageHash:     imageHash,
		ImageURLs:     info.ImageURLs,
		Title:         info.Title,
		Copyright:     info.Copyright,
		CopyrightLink: info.CopyrightLink,
		StartDate:     info.StartDate,
		FullStartDate: info.FullStartDate,
		EndDate:       info.EndDate,
		EnUSMatch:     enUSMatch,
		ExpiresAt:     getNextHourBoundary(),
	}); err != nil {
		slog.Info("Failed to cache request", "error", err)
	}

	result := &wallpaperResult{
		startDate:     info.StartDate,
		fullStartDate: info.FullStartDate,
		endDate:       info.EndDate,
		imageURLs:     info.ImageURLs,
		imageHash:     imageHash,
		colors:        colors,
		bingTitle:     info.Title,
		copyright:     info.Copyright,
		copyrightLink: info.CopyrightLink,
		title:         info.Title, // default to Bing title
		enUSMatch:     enUSMatch,
	}

	// Translation (if language requested)
	if language != "" {
		title, desc, err := app.ensureTranslation(imageHash, info.Title, info.Copyright, language)
		if err != nil {
			slog.Info("Translation failed", "error", err)
		} else {
			result.title = title
			result.description = desc
		}
	}

	return result, nil
}

// ensureColorAnalysis returns colors for the given image, using the cache or
// running AI analysis. Thread-safe via per-image mutex.
func (app *App) ensureColorAnalysis(imageData []byte, imageHash string, info *bing.WallpaperInfo) (map[string]interface{}, error) {
	if entry := app.analysisCache.Get(imageHash); entry != nil {
		slog.Info("Color analysis cache hit", "hash", imageHash)
		return entry.Colors, nil
	}

	mu := app.analysisCache.GetMutex(imageHash)
	mu.Lock()
	defer mu.Unlock()
	defer app.analysisCache.ReleaseMutex(imageHash)

	// Double-check after lock
	if entry := app.analysisCache.Get(imageHash); entry != nil {
		return entry.Colors, nil
	}

	slog.Info("Starting color analysis", "hash", imageHash)
	colors, err := app.aiAnalyzer.AnalyzeColors(imageData, imageHash, info.Title, info.Copyright)
	if err != nil {
		return nil, err
	}

	if err := app.analysisCache.Set(imageHash, colors); err != nil {
		slog.Info("Failed to cache analysis", "error", err)
	}

	return colors, nil
}

// ensureTranslation returns a translated title and description, using the
// cache or running a lightweight text-only AI call.
func (app *App) ensureTranslation(imageHash, title, copyright, language string) (string, string, error) {
	if entry := app.translationCache.Get(imageHash, language); entry != nil {
		slog.Info("Translation cache hit", "hash", imageHash, "language", language)
		return entry.Title, entry.Description, nil
	}

	mu := app.translationCache.GetMutex(imageHash, language)
	mu.Lock()
	defer mu.Unlock()
	defer app.translationCache.ReleaseMutex(imageHash, language)

	// Double-check after lock
	if entry := app.translationCache.Get(imageHash, language); entry != nil {
		return entry.Title, entry.Description, nil
	}

	slog.Info("Starting translation", "hash", imageHash, "language", language)
	result, err := app.aiAnalyzer.TranslateMetadata(title, copyright, language)
	if err != nil {
		return "", "", err
	}

	if err := app.translationCache.Set(imageHash, language, result.Title, result.Description); err != nil {
		slog.Info("Failed to cache translation", "error", err)
	}

	return result.Title, result.Description, nil
}

// =============================================================================
// Validation
// =============================================================================

func validateDaysAgo(param string) (int, error) {
	if param == "" {
		return 0, nil
	}

	daysAgo, err := strconv.Atoi(param)
	if err != nil {
		return 0, fmt.Errorf("invalid daysAgo parameter, must be an integer")
	}

	if daysAgo < 0 {
		return 0, fmt.Errorf("daysAgo cannot be negative")
	}
	if daysAgo > maxDaysBack {
		return 0, fmt.Errorf("daysAgo too large, Bing only keeps wallpapers for the last %d days", maxDaysBack)
	}

	return daysAgo, nil
}

// validateRegion checks the region query param against the instance's allowed list.
// Returns the default region if the param is empty. Comparison is case-insensitive;
// the canonical casing from the config is returned.
func (app *App) validateRegion(param string) (string, error) {
	param = strings.TrimSpace(param)
	if param == "" {
		return app.defaultRegion, nil
	}

	for _, allowed := range app.allowedRegions {
		if strings.EqualFold(param, allowed) {
			return allowed, nil
		}
	}

	return "", fmt.Errorf("region %q is not enabled on this instance. Available: %s",
		param, strings.Join(app.allowedRegions, ", "))
}

// validateLanguage checks the language query param against the instance's allowed list.
// Returns the default language if the param is empty.
func (app *App) validateLanguage(param string) (string, error) {
	param = strings.TrimSpace(param)
	if param == "" {
		return app.defaultLanguage, nil
	}

	if len(app.allowedLanguages) == 0 {
		return "", fmt.Errorf("translation is not enabled on this instance")
	}

	for _, allowed := range app.allowedLanguages {
		if strings.EqualFold(param, allowed) {
			return allowed, nil // return canonical casing from config
		}
	}

	return "", fmt.Errorf("language %q is not enabled on this instance. Available: %s",
		param, strings.Join(app.allowedLanguages, ", "))
}

// =============================================================================
// Helpers
// =============================================================================

// parseCSVEnv reads a comma-separated env var, trims whitespace, returns
// the fallback slice if the var is unset or empty.
func parseCSVEnv(key, fallback string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		if fallback == "" {
			return nil
		}
		return []string{fallback}
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}

	return out
}

func getNextHourBoundary() time.Time {
	return time.Now().Truncate(time.Hour).Add(time.Hour)
}

func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	respondWithJSON(w, statusCode, ErrorResponse{Error: message})
}

// =============================================================================
// Landing page & health
// =============================================================================

func handleLandingPage(w http.ResponseWriter, r *http.Request) {
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
</head>
<body>
    <h1>dailyhues</h1>
    <p class="subtitle">AI-extracted color palettes from Bing's daily wallpaper</p>

    <div class="code-block">
        <button class="copy-btn" onclick="copyCode()">Copy</button>
        <pre><code>curl <a href="https://dailyhues.up.railway.app/api/v2/colors">https://dailyhues.up.railway.app/api/v2/colors</a></code></pre>
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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	})
}
