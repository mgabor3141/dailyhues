package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mgabor3141/wallpaper-highlight/bing"
	"github.com/mgabor3141/wallpaper-highlight/cache"
)

// TestHandleGetColors_InvalidDaysAgo tests invalid daysAgo values
func TestHandleGetColors_InvalidDaysAgo(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)

	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
		bingClient:    bing.NewClient(defaultLocale),
	}

	tests := []struct {
		name    string
		daysAgo string
	}{
		{"Not a number", "invalid"},
		{"Negative", "-1"},
		{"Too large", "10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/colors?daysAgo="+tt.daysAgo, nil)
			w := httptest.NewRecorder()

			app.handleGetColors(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}

// TestHandleGetColors_DaysAgoTooLarge tests that daysAgo > 7 is rejected
func TestHandleGetColors_DaysAgoTooLarge(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)

	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
		bingClient:    bing.NewClient(defaultLocale),
	}

	req := httptest.NewRequest("GET", "/api/colors?daysAgo=8", nil)
	w := httptest.NewRecorder()

	app.handleGetColors(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for daysAgo too large, got %d", w.Code)
	}
}

// TestHandleGetColors_ValidDaysAgo tests valid daysAgo values
func TestHandleGetColors_ValidDaysAgo(t *testing.T) {
	tests := []struct {
		name    string
		daysAgo string
		want    int
	}{
		{"Today (empty)", "", 0},
		{"Today (explicit)", "0", 0},
		{"Yesterday", "1", 1},
		{"Last week", "7", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateDaysAgo(tt.daysAgo)
			if err != nil {
				t.Errorf("Expected no error for valid daysAgo, got: %v", err)
			}
			if result != tt.want {
				t.Errorf("Expected %d, got %d", tt.want, result)
			}
		})
	}
}

// TestHandleGetColors_InvalidLocale tests invalid locale values
func TestHandleGetColors_InvalidLocale(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)

	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
		bingClient:    bing.NewClient(defaultLocale),
	}

	tests := []struct {
		name   string
		locale string
	}{
		{"Random text", "invalid"},
		{"Wrong format", "en_US"},
		{"Unsupported", "xx-XX"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/colors?locale="+tt.locale, nil)
			w := httptest.NewRecorder()

			app.handleGetColors(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for invalid locale, got %d", w.Code)
			}
		})
	}
}

// TestHandleGetColors_ValidLocales tests that all allowed locales are accepted
func TestHandleGetColors_ValidLocales(t *testing.T) {
	validLocales := []string{
		"en-US", "en-GB", "en-CA", "en-AU", "en-IN",
		"ja-JP", "zh-CN", "zh-TW", "de-DE", "fr-FR",
		"es-ES", "it-IT", "pt-BR", "ru-RU", "ko-KR",
	}

	for _, locale := range validLocales {
		t.Run(locale, func(t *testing.T) {
			found := false
			for _, allowed := range allowedLocales {
				if locale == allowed {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Locale %s should be allowed but is not in allowedLocales slice", locale)
			}
		})
	}
}

// TestHandleGetColors_WrongMethod tests that non-GET methods are rejected
func TestHandleGetColors_WrongMethod(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)

	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
		bingClient:    bing.NewClient(defaultLocale),
	}

	methods := []string{"POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/colors", nil)
			w := httptest.NewRecorder()

			app.handleGetColors(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405 for %s method, got %d", method, w.Code)
			}
		})
	}
}

// TestHandleHealth tests the health check endpoint
func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

// TestConcurrency_ImageHashMutex tests that mutex is keyed by image hash, not daysAgo+locale
func TestConcurrency_ImageHashMutex(t *testing.T) {
	tmpDir := t.TempDir()
	analysisCache, err := cache.NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create analysis cache: %v", err)
	}

	imageHash := "hash123456789012345678901234567890123456789012345678901234567"

	// Get mutex twice for same image hash
	mu1 := analysisCache.GetMutex(imageHash)
	mu2 := analysisCache.GetMutex(imageHash)

	// Should be the exact same mutex instance
	if mu1 != mu2 {
		t.Error("Expected same mutex instance for same image hash")
	}

	// Different image hash should get different mutex
	mu3 := analysisCache.GetMutex("different890123456789012345678901234567890123456789012345678")
	if mu1 == mu3 {
		t.Error("Expected different mutex instance for different image hash")
	}
}

// TestConcurrency_TwoLevelCacheSystem tests the new two-level cache behavior
func TestConcurrency_TwoLevelCacheSystem(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, err := cache.NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create request cache: %v", err)
	}

	analysisCache, err := cache.NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create analysis cache: %v", err)
	}

	daysAgo := 0
	imageHash := "shared789012345678901234567890123456789012345678901234567890"

	// Simulate: Same image used by both en-US and ja-JP
	imageURLs := map[string]string{"1920x1080": "https://bing.com/image.jpg"}
	colors := map[string]string{"highlight": "#FF0000", "primary": "#00FF00"}
	title := "Test Title"
	copyright := "Test Copyright © Photographer"
	copyrightLink := "https://example.com/test"
	startDate := "20251019"
	fullStartDate := "202510190700"
	endDate := "20251020"
	expiresAt := getNextHourBoundary()

	// Store analysis once (shared)
	err = analysisCache.Set(imageHash, colors)
	if err != nil {
		t.Fatalf("Failed to set analysis: %v", err)
	}

	// Store request metadata for en-US
	err = requestCache.Set("en-US", daysAgo, imageHash, imageURLs, title, copyright, copyrightLink, startDate, fullStartDate, endDate, expiresAt)
	if err != nil {
		t.Fatalf("Failed to set en-US request: %v", err)
	}

	// Store request metadata for ja-JP (same image hash!)
	err = requestCache.Set("ja-JP", daysAgo, imageHash, imageURLs, title, copyright, copyrightLink, startDate, fullStartDate, endDate, expiresAt)
	if err != nil {
		t.Fatalf("Failed to set ja-JP request: %v", err)
	}

	// Both requests should point to same analysis
	reqUS := requestCache.Get("en-US", daysAgo)
	reqJP := requestCache.Get("ja-JP", daysAgo)

	if reqUS == nil || reqJP == nil {
		t.Fatal("Expected both request entries to exist")
	}

	if reqUS.ImageHash != reqJP.ImageHash {
		t.Error("Expected both requests to share same image hash")
	}

	// Both should get same analysis
	analysisUS := analysisCache.Get(reqUS.ImageHash)
	analysisJP := analysisCache.Get(reqJP.ImageHash)

	if analysisUS != analysisJP {
		t.Error("Expected both locales to get same analysis instance")
	}
}
