package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mgabor3141/wallpaper-highlight/cache"
)

// TestHandleGetColors_InvalidDateFormat tests invalid date formats
func TestHandleGetColors_InvalidDateFormat(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)

	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
	}

	tests := []struct {
		name string
		date string
	}{
		{"Invalid format", "2024-13-45"},
		{"Wrong separator", "2024/01/15"},
		{"No dashes", "20240115"},
		{"Text", "invalid"},
		{"Incomplete", "2024-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/colors?date="+tt.date, nil)
			w := httptest.NewRecorder()

			app.handleGetColors(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}

// TestHandleGetColors_FutureDate tests that future dates are rejected
func TestHandleGetColors_FutureDate(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)

	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
	}

	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	req := httptest.NewRequest("GET", "/api/colors?date="+tomorrow, nil)
	w := httptest.NewRecorder()

	app.handleGetColors(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for future date, got %d", w.Code)
	}
}

// TestHandleGetColors_DateTooOld tests that dates older than 7 days are rejected
func TestHandleGetColors_DateTooOld(t *testing.T) {
	tmpDir := t.TempDir()
	requestCache, _ := cache.NewRequestCache(tmpDir)
	analysisCache, _ := cache.NewAnalysisCache(tmpDir)

	app := &App{
		requestCache:  requestCache,
		analysisCache: analysisCache,
	}

	eightDaysAgo := time.Now().AddDate(0, 0, -8).Format("2006-01-02")

	req := httptest.NewRequest("GET", "/api/colors?date="+eightDaysAgo, nil)
	w := httptest.NewRecorder()

	app.handleGetColors(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for date too old, got %d", w.Code)
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
			if !allowedLocales[locale] {
				t.Errorf("Locale %s should be allowed but is not in allowedLocales map", locale)
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

// TestConcurrency_ImageHashMutex tests that mutex is keyed by image hash, not date+locale
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

	date := "2024-01-15"
	imageHash := "shared789012345678901234567890123456789012345678901234567890"

	// Simulate: Same image used by both en-US and ja-JP
	imageURLs := map[string]string{"1920x1080": "https://bing.com/image.jpg"}
	colors := map[string]string{"highlight": "#FF0000", "primary": "#00FF00"}
	copyright := "Test Copyright © Photographer"
	copyrightLink := "https://example.com/test"

	// Store analysis once (shared)
	err = analysisCache.Set(imageHash, colors)
	if err != nil {
		t.Fatalf("Failed to set analysis: %v", err)
	}

	// Store request metadata for en-US
	err = requestCache.Set(date, "en-US", imageHash, imageURLs, copyright, copyrightLink)
	if err != nil {
		t.Fatalf("Failed to set en-US request: %v", err)
	}

	// Store request metadata for ja-JP (same image hash!)
	err = requestCache.Set(date, "ja-JP", imageHash, imageURLs, copyright, copyrightLink)
	if err != nil {
		t.Fatalf("Failed to set ja-JP request: %v", err)
	}

	// Both requests should point to same analysis
	reqUS := requestCache.Get(date, "en-US")
	reqJP := requestCache.Get(date, "ja-JP")

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
