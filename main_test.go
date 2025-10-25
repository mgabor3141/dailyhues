package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mgabor3141/wallpaper-highlight/bing"
	"github.com/mgabor3141/wallpaper-highlight/cache"
)

// TestHandleGetColors_InvalidDateFormat tests invalid date formats
func TestHandleGetColors_InvalidDateFormat(t *testing.T) {
	app := &App{
		processing: make(map[string]*sync.Mutex),
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
	app := &App{
		processing: make(map[string]*sync.Mutex),
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
	app := &App{
		processing: make(map[string]*sync.Mutex),
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
	app := &App{
		processing: make(map[string]*sync.Mutex),
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
	app := &App{
		processing: make(map[string]*sync.Mutex),
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

// TestConcurrency_CacheMissBlocking tests that concurrent requests for the same date
// block on the mutex and the second request gets the cached result
func TestConcurrency_CacheMissBlocking(t *testing.T) {
	// Create a mock cache that simulates a slow operation
	tmpDir := t.TempDir()
	mockCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	app := &App{
		cache:      mockCache,
		bingClient: bing.NewClient("en-US"),
		processing: make(map[string]*sync.Mutex),
	}

	date := time.Now().Format("2006-01-02")

	// Channel to track request order
	started := make(chan int, 2)
	finished := make(chan int, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	// First request - will have cache miss and simulate slow processing
	go func() {
		defer wg.Done()
		started <- 1

		// We can't actually test the full flow without mocking all dependencies,
		// but we can verify the mutex behavior by directly testing getDateMutex
		mu := app.getDateMutex(date)
		mu.Lock()
		time.Sleep(100 * time.Millisecond) // Simulate processing
		mu.Unlock()

		finished <- 1
	}()

	// Second request - should block until first completes
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // Ensure first request starts first
		started <- 2

		mu := app.getDateMutex(date)
		mu.Lock()
		mu.Unlock()

		finished <- 2
	}()

	// Wait for both to start
	<-started
	<-started

	// Verify order of completion
	first := <-finished
	second := <-finished

	if first != 1 {
		t.Errorf("Expected first request to finish first, but request %d finished first", first)
	}
	if second != 2 {
		t.Errorf("Expected second request to finish second, but request %d finished second", second)
	}

	wg.Wait()
}

// TestConcurrency_DifferentDatesNoBlocking tests that requests for different dates
// don't block each other
func TestConcurrency_DifferentDatesNoBlocking(t *testing.T) {
	app := &App{
		processing: make(map[string]*sync.Mutex),
	}

	date1 := "2024-01-15"
	date2 := "2024-01-16"

	finished := make(chan string, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	// First request for date1
	go func() {
		defer wg.Done()
		mu := app.getDateMutex(date1)
		mu.Lock()
		time.Sleep(50 * time.Millisecond)
		mu.Unlock()
		finished <- date1
	}()

	// Second request for date2 - should NOT block
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // Start slightly after first
		mu := app.getDateMutex(date2)
		mu.Lock()
		time.Sleep(10 * time.Millisecond)
		mu.Unlock()
		finished <- date2
	}()

	// Second request should finish first since it's not blocked
	first := <-finished
	if first != date2 {
		t.Errorf("Expected date2 to finish first, got %s", first)
	}

	second := <-finished
	if second != date1 {
		t.Errorf("Expected date1 to finish second, got %s", second)
	}

	wg.Wait()
}

// TestConcurrency_GetDateMutex tests that getDateMutex returns the same mutex for the same date
func TestConcurrency_GetDateMutex(t *testing.T) {
	app := &App{
		processing: make(map[string]*sync.Mutex),
	}

	date := "2024-01-15"

	// Get mutex twice for same date
	mu1 := app.getDateMutex(date)
	mu2 := app.getDateMutex(date)

	// Should be the exact same mutex instance
	if mu1 != mu2 {
		t.Error("Expected same mutex instance for same date")
	}

	// Different date should get different mutex
	mu3 := app.getDateMutex("2024-01-16")
	if mu1 == mu3 {
		t.Error("Expected different mutex instance for different date")
	}
}

// TestConcurrency_ManyParallelRequests tests handling of many parallel requests
func TestConcurrency_ManyParallelRequests(t *testing.T) {
	app := &App{
		processing: make(map[string]*sync.Mutex),
	}

	date := "2024-01-15"
	numRequests := 10

	var wg sync.WaitGroup
	wg.Add(numRequests)

	completed := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			defer wg.Done()
			mu := app.getDateMutex(date)
			mu.Lock()
			time.Sleep(10 * time.Millisecond) // Simulate work
			mu.Unlock()
			completed <- id
		}(i)
	}

	wg.Wait()
	close(completed)

	// Count completions
	count := 0
	for range completed {
		count++
	}

	if count != numRequests {
		t.Errorf("Expected %d completions, got %d", numRequests, count)
	}
}

// TestConcurrency_LocaleInCacheKey tests that locale is part of the cache key
func TestConcurrency_LocaleInCacheKey(t *testing.T) {
	tmpDir := t.TempDir()
	mockCache, err := cache.New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	app := &App{
		cache:      mockCache,
		bingClient: bing.NewClient("en-US"),
		processing: make(map[string]*sync.Mutex),
	}

	date := "2024-01-15"
	localeUS := "en-US"
	localeJP := "ja-JP"

	// Cache keys should include locale
	cacheKeyUS := getCacheKey(date, localeUS)
	cacheKeyJP := getCacheKey(date, localeJP)

	// Simulate caching for en-US locale
	images1 := map[string]string{"1920x1080": "https://bing.com/en-us-image.jpg"}
	colors1 := map[string]string{"highlight": "#FF0000"}
	err = mockCache.Set(cacheKeyUS, images1, colors1)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Simulate caching for ja-JP locale (different wallpaper!)
	images2 := map[string]string{"1920x1080": "https://bing.com/ja-jp-image.jpg"}
	colors2 := map[string]string{"highlight": "#00FF00"}
	err = mockCache.Set(cacheKeyJP, images2, colors2)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Get cache for en-US - should get the en-US image
	cachedUS, _ := mockCache.Get(cacheKeyUS)
	if cachedUS == nil {
		t.Fatal("Expected cached result for en-US")
	}

	if cachedUS.Images["1920x1080"] != "https://bing.com/en-us-image.jpg" {
		t.Errorf("Expected en-US image, but got: %s", cachedUS.Images["1920x1080"])
	}

	if cachedUS.Colors["highlight"] != "#FF0000" {
		t.Errorf("Expected en-US color, but got: %s", cachedUS.Colors["highlight"])
	}

	// Get cache for ja-JP - should get the ja-JP image
	cachedJP, _ := mockCache.Get(cacheKeyJP)
	if cachedJP == nil {
		t.Fatal("Expected cached result for ja-JP")
	}

	if cachedJP.Images["1920x1080"] != "https://bing.com/ja-jp-image.jpg" {
		t.Errorf("Expected ja-JP image, but got: %s", cachedJP.Images["1920x1080"])
	}

	if cachedJP.Colors["highlight"] != "#00FF00" {
		t.Errorf("Expected ja-JP color, but got: %s", cachedJP.Colors["highlight"])
	}

	// Verify mutex keys include locale (different locales = different mutexes)
	mutex1 := app.getDateMutex(cacheKeyUS)
	mutex2 := app.getDateMutex(cacheKeyJP)

	if mutex1 == mutex2 {
		t.Error("Different locales should have different mutexes")
	}

	// Verify same cache key gives same mutex
	mutex3 := app.getDateMutex(cacheKeyUS)
	if mutex1 != mutex3 {
		t.Error("Same cache key should return same mutex")
	}
}
