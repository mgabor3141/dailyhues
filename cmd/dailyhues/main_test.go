package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mgabor3141/dailyhues/internal/bing"
	"github.com/mgabor3141/dailyhues/internal/cache"
)

// newTestApp creates an App with test defaults for use in handler tests
func newTestApp(t *testing.T) *App {
	t.Helper()
	tmpDir := t.TempDir()

	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)
	translationCache, _ := cache.NewTranslationCache(tmpDir)

	return &App{
		requestCache:     requestCache,
		analysisCache:    analysisCache,
		translationCache: translationCache,
		bingClient:       bing.NewClient("en-US"),
		allowedRegions:   []string{"global", "en-US"},
		allowedLanguages: []string{"English"},
		defaultRegion:    "global",
		defaultLanguage:  "English",
	}
}

// --- daysAgo validation ---

func TestValidateDaysAgo_Invalid(t *testing.T) {
	tests := []struct {
		name string
		param string
	}{
		{"Not a number", "invalid"},
		{"Negative", "-1"},
		{"Too large", "10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateDaysAgo(tt.param)
			if err == nil {
				t.Error("Expected error for invalid daysAgo")
			}
		})
	}
}

func TestValidateDaysAgo_Valid(t *testing.T) {
	tests := []struct {
		name string
		param string
		want int
	}{
		{"Empty (default)", "", 0},
		{"Today", "0", 0},
		{"Yesterday", "1", 1},
		{"Last week", "7", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateDaysAgo(tt.param)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.want {
				t.Errorf("Got %d, want %d", result, tt.want)
			}
		})
	}
}

// --- region validation ---

func TestValidateRegion(t *testing.T) {
	app := &App{
		allowedRegions: []string{"global", "en-US", "ja-JP"},
		defaultRegion:  "global",
	}

	tests := []struct {
		name    string
		param   string
		want    string
		wantErr bool
	}{
		{"Empty returns default", "", "global", false},
		{"Global allowed", "global", "global", false},
		{"en-US allowed", "en-US", "en-US", false},
		{"ja-JP allowed", "ja-JP", "ja-JP", false},
		{"de-DE not allowed", "de-DE", "", true},
		{"Random text", "foobar", "", true},
		{"Whitespace trimmed", "  global  ", "global", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := app.validateRegion(tt.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRegion(%q) error = %v, wantErr %v", tt.param, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateRegion(%q) = %q, want %q", tt.param, got, tt.want)
			}
		})
	}
}

// --- language validation ---

func TestValidateLanguage(t *testing.T) {
	app := &App{
		allowedLanguages: []string{"English", "Japanese"},
		defaultLanguage:  "English",
	}

	tests := []struct {
		name    string
		param   string
		want    string
		wantErr bool
	}{
		{"Empty returns default", "", "English", false},
		{"English allowed", "English", "English", false},
		{"Japanese allowed", "Japanese", "Japanese", false},
		{"Case insensitive", "english", "English", false},
		{"Case insensitive JP", "japanese", "Japanese", false},
		{"German not allowed", "German", "", true},
		{"Random text", "foobar", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := app.validateLanguage(tt.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLanguage(%q) error = %v, wantErr %v", tt.param, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validateLanguage(%q) = %q, want %q", tt.param, got, tt.want)
			}
		})
	}
}

func TestValidateLanguage_NoLanguagesConfigured(t *testing.T) {
	app := &App{
		allowedLanguages: nil,
		defaultLanguage:  "",
	}

	// Empty param should return empty (no translation)
	got, err := app.validateLanguage("")
	if err != nil {
		t.Errorf("Empty param with no languages should not error, got: %v", err)
	}
	if got != "" {
		t.Errorf("Expected empty string, got %q", got)
	}

	// Explicit language should be rejected
	_, err = app.validateLanguage("English")
	if err == nil {
		t.Error("Expected error when requesting language on instance with no languages configured")
	}
}

// --- handler method checks ---

func TestV1_WrongMethod(t *testing.T) {
	app := newTestApp(t)

	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/colors", nil)
			w := httptest.NewRecorder()
			app.handleGetColors(w, req)
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected 405, got %d", w.Code)
			}
		})
	}
}

func TestV2_WrongMethod(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("POST", "/api/v2/colors", nil)
	w := httptest.NewRecorder()
	app.handleGetColorsV2(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

func TestV2_InvalidDaysAgo(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("GET", "/api/v2/colors?daysAgo=invalid", nil)
	w := httptest.NewRecorder()
	app.handleGetColorsV2(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestV2_DisallowedRegion(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("GET", "/api/v2/colors?region=de-DE", nil)
	w := httptest.NewRecorder()
	app.handleGetColorsV2(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestV2_DisallowedLanguage(t *testing.T) {
	app := newTestApp(t)

	req := httptest.NewRequest("GET", "/api/v2/colors?language=Japanese", nil)
	w := httptest.NewRecorder()
	app.handleGetColorsV2(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

// --- health endpoint ---

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected application/json, got %s", ct)
	}
}

// --- cache sharing ---

func TestConcurrency_ImageHashMutex(t *testing.T) {
	tmpDir := t.TempDir()
	analysisCache, err := cache.NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create analysis cache: %v", err)
	}

	hash := "hash123456789012345678901234567890123456789012345678901234567"

	mu1 := analysisCache.GetMutex(hash)
	mu2 := analysisCache.GetMutex(hash)
	if mu1 != mu2 {
		t.Error("Same hash should return same mutex")
	}

	mu3 := analysisCache.GetMutex("different890123456789012345678901234567890123456789012345678")
	if mu1 == mu3 {
		t.Error("Different hash should return different mutex")
	}
}

func TestConcurrency_AnalysisCacheSharedAcrossRegions(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)

	imageHash := "shared789012345678901234567890123456789012345678901234567890"
	imageURLs := map[string]string{"1920x1080": "https://bing.com/image.jpg"}
	colors := map[string]interface{}{"gradient_from": "#FF0000"}

	// Store analysis once (shared)
	if err := analysisCache.Set(imageHash, colors); err != nil {
		t.Fatalf("Failed to set analysis: %v", err)
	}

	// Store request metadata for two different regions pointing to same image
	for _, region := range []string{"global", "en-US"} {
		if err := requestCache.Set(&cache.RequestEntry{
			Locale:    region,
			DaysAgo:   0,
			ImageHash: imageHash,
			ImageURLs: imageURLs,
			Title:     "Test",
			ExpiresAt: getNextHourBoundary(),
		}); err != nil {
			t.Fatalf("Failed to set request for %s: %v", region, err)
		}
	}

	// Both should resolve to the same analysis
	reqGlobal := requestCache.Get("global", 0)
	reqEnUS := requestCache.Get("en-US", 0)

	if reqGlobal.ImageHash != reqEnUS.ImageHash {
		t.Error("Both regions should share the same image hash")
	}

	a1 := analysisCache.Get(reqGlobal.ImageHash)
	a2 := analysisCache.Get(reqEnUS.ImageHash)
	if a1 != a2 {
		t.Error("Both regions should get the same analysis instance")
	}
}

// --- parseCSVEnv ---

func TestParseCSVEnv(t *testing.T) {
	// Test fallback when env var is not set
	result := parseCSVEnv("NONEXISTENT_VAR_FOR_TEST_12345", "default")
	if len(result) != 1 || result[0] != "default" {
		t.Errorf("Expected [default], got %v", result)
	}

	// Test empty fallback
	result = parseCSVEnv("NONEXISTENT_VAR_FOR_TEST_12345", "")
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}
}
