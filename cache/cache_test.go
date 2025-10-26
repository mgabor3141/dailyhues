package cache

import (
	"sync"
	"testing"
	"time"
)

// TestRequestCache_New tests request cache initialization
func TestRequestCache_New(t *testing.T) {
	tmpDir := t.TempDir()

	cache, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create request cache: %v", err)
	}

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

// TestRequestCache_SetAndGet tests basic request cache operations
func TestRequestCache_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	locale := "en-US"
	daysAgo := 0
	imageHash := "abc123def456789012345678901234567890123456789012345678901234"
	imageURLs := map[string]string{
		"1920x1080": "https://bing.com/image_1920x1080.jpg",
		"1280x720":  "https://bing.com/image_1280x720.jpg",
	}
	title := "Test Title"
	copyright := "Test Copyright © Photographer"
	copyrightLink := "https://example.com/copyright"
	expiresAt := time.Now().Add(time.Hour)

	// Set cache
	startDate := "20251019"
	fullStartDate := "202510190700"
	endDate := "20251020"
	err = cache.Set(locale, daysAgo, imageHash, imageURLs, title, copyright, copyrightLink, startDate, fullStartDate, endDate, expiresAt)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Get cache
	entry := cache.Get(locale, daysAgo)
	if entry == nil {
		t.Fatal("Expected entry, got nil")
	}

	if entry.DaysAgo != daysAgo {
		t.Errorf("Expected daysAgo %d, got %d", daysAgo, entry.DaysAgo)
	}

	if entry.Locale != locale {
		t.Errorf("Expected locale %s, got %s", locale, entry.Locale)
	}

	if entry.ImageHash != imageHash {
		t.Errorf("Expected imageHash %s, got %s", imageHash, entry.ImageHash)
	}

	if len(entry.ImageURLs) != len(imageURLs) {
		t.Errorf("Expected %d URLs, got %d", len(imageURLs), len(entry.ImageURLs))
	}
}

// TestRequestCache_GetNonExistent tests getting a non-existent entry
func TestRequestCache_GetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	entry := cache.Get("xx-XX", 99)
	if entry != nil {
		t.Error("Expected nil for non-existent entry")
	}
}

// TestRequestCache_Persistence tests that cache survives across instances
func TestRequestCache_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first cache instance and set data
	cache1, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create first cache: %v", err)
	}

	locale := "ja-JP"
	daysAgo := 1
	imageHash := "def789ghi012345678901234567890123456789012345678901234567890"
	imageURLs := map[string]string{"1920x1080": "https://bing.com/img.jpg"}
	title := "Test Title"
	copyright := "Persistent Test Copyright"
	copyrightLink := "https://example.com/persist"
	expiresAt := time.Now().Add(time.Hour)

	startDate := "20251019"
	fullStartDate := "202510190700"
	endDate := "20251020"
	err = cache1.Set(locale, daysAgo, imageHash, imageURLs, title, copyright, copyrightLink, startDate, fullStartDate, endDate, expiresAt)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Create second cache instance (simulates restart)
	cache2, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second cache: %v", err)
	}

	// Load from disk
	err = cache2.LoadAll()
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	// Data should be loaded from file
	entry := cache2.Get(locale, daysAgo)
	if entry == nil {
		t.Fatal("Expected entry from persisted file, got nil")
	}

	if entry.ImageHash != imageHash {
		t.Errorf("Expected imageHash %s, got %s", imageHash, entry.ImageHash)
	}
}

// TestRequestCache_TTLExpiration tests that cache entries respect expiration time
func TestRequestCache_TTLExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	locale := "en-US"
	daysAgo := 0
	imageHash := "ttl12345678901234567890123456789012345678901234567890123456"
	imageURLs := map[string]string{"1920x1080": "https://bing.com/img.jpg"}
	title := "TTL Test Title"
	copyright := "TTL Test Copyright"
	copyrightLink := "https://example.com/ttl"

	// Test 1: Entry with expiration in the past
	startDate := "20251019"
	fullStartDate := "202510190700"
	endDate := "20251020"
	pastExpiration := time.Now().Add(-1 * time.Hour)
	err = cache.Set(locale, daysAgo, imageHash, imageURLs, title, copyright, copyrightLink, startDate, fullStartDate, endDate, pastExpiration)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	entry := cache.Get(locale, daysAgo)
	if entry == nil {
		t.Fatal("Expected entry to exist")
	}

	if time.Now().Before(entry.ExpiresAt) {
		t.Error("Expected entry to be expired")
	}

	// Test 2: Entry with expiration in the future
	futureExpiration := time.Now().Add(1 * time.Hour)
	err = cache.Set(locale, daysAgo, imageHash, imageURLs, title, copyright, copyrightLink, startDate, fullStartDate, endDate, futureExpiration)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	entry = cache.Get(locale, daysAgo)
	if entry == nil {
		t.Fatal("Expected entry to exist")
	}

	if !time.Now().Before(entry.ExpiresAt) {
		t.Error("Expected entry to not be expired")
	}

	// Test 3: Verify expiration time is persisted
	cache2, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second cache: %v", err)
	}

	err = cache2.LoadAll()
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	entry2 := cache2.Get(locale, daysAgo)
	if entry2 == nil {
		t.Fatal("Expected entry to exist after reload")
	}

	if !entry.ExpiresAt.Equal(entry2.ExpiresAt) {
		t.Errorf("Expected expiration time %v, got %v", entry.ExpiresAt, entry2.ExpiresAt)
	}
}

// TestRequestCache_ConcurrentAccess tests thread safety
func TestRequestCache_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				locale := "en-US"
				daysAgo := 0
				imageHash := "abc123test456789012345678901234567890123456789012345678901"
				imageURLs := map[string]string{"1920x1080": "https://bing.com/img.jpg"}
				title := "Test Title"
				copyright := "Concurrent Test Copyright"
				copyrightLink := "https://example.com/concurrent"
				expiresAt := time.Now().Add(time.Hour)

				// Write
				startDate := "20251019"
				fullStartDate := "202510190700"
				endDate := "20251020"
				err := cache.Set(locale, daysAgo, imageHash, imageURLs, title, copyright, copyrightLink, startDate, fullStartDate, endDate, expiresAt)
				if err != nil {
					t.Errorf("Goroutine %d: Failed to set cache: %v", id, err)
					return
				}

				// Read
				_ = cache.Get(locale, daysAgo)
			}
		}(i)
	}

	wg.Wait()
}

// TestAnalysisCache_New tests analysis cache initialization
func TestAnalysisCache_New(t *testing.T) {
	tmpDir := t.TempDir()

	cache, err := NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create analysis cache: %v", err)
	}

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}
}

// TestAnalysisCache_SetAndGet tests basic analysis cache operations
func TestAnalysisCache_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	imageHash := "fedcba987654321098765432109876543210987654321098765432109876"
	colors := map[string]string{
		"highlight": "#ff0000",
		"primary":   "#00ff00",
		"secondary": "#0000ff",
	}

	// Set cache
	err = cache.Set(imageHash, colors)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Get cache
	entry := cache.Get(imageHash)
	if entry == nil {
		t.Fatal("Expected entry, got nil")
	}

	if entry.ImageHash != imageHash {
		t.Errorf("Expected imageHash %s, got %s", imageHash, entry.ImageHash)
	}

	if len(entry.Colors) != len(colors) {
		t.Errorf("Expected %d colors, got %d", len(colors), len(entry.Colors))
	}

	for name, color := range colors {
		if entry.Colors[name] != color {
			t.Errorf("Expected color %s for %s, got %s", color, name, entry.Colors[name])
		}
	}
}

// TestAnalysisCache_GetNonExistent tests getting a non-existent entry
func TestAnalysisCache_GetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	entry := cache.Get("NonExistentImageID")
	if entry != nil {
		t.Error("Expected nil for non-existent entry")
	}
}

// TestAnalysisCache_Persistence tests that cache survives across instances
func TestAnalysisCache_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first cache instance and set data
	cache1, err := NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create first cache: %v", err)
	}

	imageHash := "persistent1234567890123456789012345678901234567890123456789012"
	colors := map[string]string{"highlight": "#aabbcc", "primary": "#ddeeff"}

	err = cache1.Set(imageHash, colors)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Create second cache instance (simulates restart)
	cache2, err := NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second cache: %v", err)
	}

	// Load from disk
	err = cache2.LoadAll()
	if err != nil {
		t.Fatalf("Failed to load cache: %v", err)
	}

	// Data should be loaded from file
	entry := cache2.Get(imageHash)
	if entry == nil {
		t.Fatal("Expected entry from persisted file, got nil")
	}

	if len(entry.Colors) != len(colors) {
		t.Errorf("Expected %d colors, got %d", len(colors), len(entry.Colors))
	}
}

// TestAnalysisCache_GetMutex tests per-image mutex locking
func TestAnalysisCache_GetMutex(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	imageHash := "hash123456789012345678901234567890123456789012345678901234567"

	// Get mutex twice for same image
	mu1 := cache.GetMutex(imageHash)
	mu2 := cache.GetMutex(imageHash)

	// Should be the exact same mutex instance
	if mu1 != mu2 {
		t.Error("Expected same mutex instance for same image hash")
	}

	// Different image should get different mutex
	mu3 := cache.GetMutex("different890123456789012345678901234567890123456789012345678")
	if mu1 == mu3 {
		t.Error("Expected different mutex instance for different image hash")
	}
}

// TestAnalysisCache_ConcurrentBlocking tests that concurrent requests for same image block
func TestAnalysisCache_ConcurrentBlocking(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	imageHash := "hash123concurrent456789012345678901234567890123456789012345"

	started := make(chan int, 2)
	finished := make(chan int, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	// First request
	go func() {
		defer wg.Done()

		mu := cache.GetMutex(imageHash)
		mu.Lock()
		started <- 1
		time.Sleep(100 * time.Millisecond) // Hold lock for meaningful time
		mu.Unlock()

		finished <- 1
	}()

	// Wait for first to acquire lock
	<-started

	// Second request - should block
	go func() {
		defer wg.Done()
		started <- 2

		mu := cache.GetMutex(imageHash)
		mu.Lock()
		mu.Unlock()

		finished <- 2
	}()

	// Wait for second to start
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

// TestAnalysisCache_SharedImageAcrossLocales tests the key benefit:
// Multiple locales with same image share the analysis result
func TestAnalysisCache_SharedImageAcrossLocales(t *testing.T) {
	tmpDir := t.TempDir()
	analysisCache, err := NewAnalysisCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create analysis cache: %v", err)
	}

	requestCache, err := NewRequestCache(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create request cache: %v", err)
	}

	// Simulate: Same image used by both en-US and ja-JP on same daysAgo
	imageHash := "shared789012345678901234567890123456789012345678901234567890"
	daysAgo := 0

	// Store analysis once (shared)
	colors := map[string]string{"highlight": "#ff0000", "primary": "#00ff00"}
	err = analysisCache.Set(imageHash, colors)
	if err != nil {
		t.Fatalf("Failed to set analysis: %v", err)
	}

	// Store request metadata for en-US
	imageURLs := map[string]string{"1920x1080": "https://bing.com/img.jpg"}
	title := "Shared Image Title"
	copyright := "Shared Image Copyright"
	copyrightLink := "https://example.com/shared"
	startDate := "20251019"
	fullStartDate := "202510190700"
	endDate := "20251020"
	expiresAt := time.Now().Add(time.Hour)
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

// BenchmarkAnalysisCache_Get benchmarks cache retrieval
func BenchmarkAnalysisCache_Get(b *testing.B) {
	tmpDir := b.TempDir()
	cache, err := NewAnalysisCache(tmpDir)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	// Pre-populate cache
	imageHash := "bench12345678901234567890123456789012345678901234567890123456"
	colors := map[string]string{"highlight": "#ff0000", "primary": "#00ff00"}
	err = cache.Set(imageHash, colors)
	if err != nil {
		b.Fatalf("Failed to set cache: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.Get(imageHash)
	}
}

// BenchmarkAnalysisCache_Set benchmarks cache storage
func BenchmarkAnalysisCache_Set(b *testing.B) {
	tmpDir := b.TempDir()
	cache, err := NewAnalysisCache(tmpDir)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	imageHash := "bench12345678901234567890123456789012345678901234567890123456"
	colors := map[string]string{"highlight": "#ff0000", "primary": "#00ff00"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cache.Set(imageHash, colors)
		if err != nil {
			b.Fatalf("Failed to set cache: %v", err)
		}
	}
}
