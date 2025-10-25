package cache

import (
	"path/filepath"
	"sync"
	"testing"
)

// TestNew tests cache initialization
func TestNew(t *testing.T) {
	tmpDir := t.TempDir() // Automatically cleaned up after test

	cache, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	if cache == nil {
		t.Fatal("Cache should not be nil")
	}

	if cache.cacheDir != tmpDir {
		t.Errorf("Expected cache dir %s, got %s", tmpDir, cache.cacheDir)
	}
}

// TestSetAndGet tests basic cache operations
func TestSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test data
	date := "2024-01-15"
	images := map[string]string{
		"1920x1080": "https://example.com/image_1920x1080.jpg",
		"1280x720":  "https://example.com/image_1280x720.jpg",
	}
	colors := map[string]string{
		"highlight": "#ff0000",
		"primary":   "#00ff00",
		"secondary": "#0000ff",
	}

	// Set cache
	err = cache.Set(date, images, colors)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Get cache
	theme, err := cache.Get(date)
	if err != nil {
		t.Fatalf("Failed to get cache: %v", err)
	}

	if theme == nil {
		t.Fatal("Expected theme, got nil")
	}

	if theme.Date != date {
		t.Errorf("Expected date %s, got %s", date, theme.Date)
	}

	if len(theme.Images) != len(images) {
		t.Errorf("Expected %d images, got %d", len(images), len(theme.Images))
	}

	if len(theme.Colors) != len(colors) {
		t.Errorf("Expected %d colors, got %d", len(colors), len(theme.Colors))
	}

	for name, color := range colors {
		if theme.Colors[name] != color {
			t.Errorf("Expected color %s for %s, got %s", color, name, theme.Colors[name])
		}
	}

	if !theme.FromCache {
		t.Error("FromCache should be true")
	}
}

// TestGetNonExistent tests getting a non-existent cache entry
func TestGetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	theme, err := cache.Get("2024-12-31")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if theme != nil {
		t.Error("Expected nil for non-existent cache entry")
	}
}

// TestFilePersistence tests that cache survives across instances
func TestFilePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first cache instance and set data
	cache1, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create first cache: %v", err)
	}

	date := "2024-01-15"
	images := map[string]string{"1920x1080": "https://example.com/img.jpg"}
	colors := map[string]string{"highlight": "#aabbcc", "primary": "#ddeeff"}

	err = cache1.Set(date, images, colors)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Create second cache instance (simulates restart)
	cache2, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create second cache: %v", err)
	}

	// Data should be loaded from file
	theme, err := cache2.Get(date)
	if err != nil {
		t.Fatalf("Failed to get cache from second instance: %v", err)
	}

	if theme == nil {
		t.Fatal("Expected theme from persisted file, got nil")
	}

	if len(theme.Colors) != len(colors) {
		t.Errorf("Expected %d colors, got %d", len(colors), len(theme.Colors))
	}
}

// TestConcurrentAccess tests thread safety with parallel goroutines
func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines doing concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				date := "2024-01-15"
				images := map[string]string{"1920x1080": "https://example.com/img.jpg"}
				colors := map[string]string{"highlight": "#111111", "primary": "#222222"}

				// Write
				err := cache.Set(date, images, colors)
				if err != nil {
					t.Errorf("Goroutine %d: Failed to set cache: %v", id, err)
					return
				}

				// Read
				_, err = cache.Get(date)
				if err != nil {
					t.Errorf("Goroutine %d: Failed to get cache: %v", id, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestCacheFileName tests filename generation
func TestCacheFileName(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	tests := []struct {
		date     string
		expected string
	}{
		{"2024-01-15", filepath.Join(tmpDir, "cache_2024-01-15.json")},
		{"2023-12-31", filepath.Join(tmpDir, "cache_2023-12-31.json")},
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			filename := cache.getFilename(tt.date)
			if filename != tt.expected {
				t.Errorf("Expected filename %s, got %s", tt.expected, filename)
			}
		})
	}
}

// TestMultipleDates tests caching multiple dates simultaneously
func TestMultipleDates(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := New(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	dates := []struct {
		date   string
		images map[string]string
		colors map[string]string
	}{
		{"2024-01-15", map[string]string{"1920x1080": "https://example.com/img1.jpg"}, map[string]string{"highlight": "#ff0000", "primary": "#00ff00"}},
		{"2024-01-16", map[string]string{"1920x1080": "https://example.com/img2.jpg"}, map[string]string{"highlight": "#0000ff", "primary": "#ffff00"}},
		{"2024-01-17", map[string]string{"1920x1080": "https://example.com/img3.jpg"}, map[string]string{"highlight": "#ff00ff", "primary": "#00ffff"}},
	}

	// Set all dates
	for _, d := range dates {
		err := cache.Set(d.date, d.images, d.colors)
		if err != nil {
			t.Fatalf("Failed to set cache for %s: %v", d.date, err)
		}
	}

	// Verify all dates
	for _, d := range dates {
		theme, err := cache.Get(d.date)
		if err != nil {
			t.Fatalf("Failed to get cache for %s: %v", d.date, err)
		}

		if theme == nil {
			t.Fatalf("Expected theme for %s, got nil", d.date)
		}

		if len(theme.Colors) != len(d.colors) {
			t.Errorf("Date %s: expected %d colors, got %d", d.date, len(d.colors), len(theme.Colors))
		}
	}
}

// BenchmarkCacheGet benchmarks cache retrieval
func BenchmarkCacheGet(b *testing.B) {
	tmpDir := b.TempDir()
	cache, err := New(tmpDir)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	// Pre-populate cache
	date := "2024-01-15"
	images := map[string]string{"1920x1080": "https://example.com/img.jpg"}
	colors := map[string]string{"highlight": "#ff0000", "primary": "#00ff00", "secondary": "#0000ff"}
	err = cache.Set(date, images, colors)
	if err != nil {
		b.Fatalf("Failed to set cache: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cache.Get(date)
		if err != nil {
			b.Fatalf("Failed to get cache: %v", err)
		}
	}
}

// BenchmarkCacheSet benchmarks cache storage
func BenchmarkCacheSet(b *testing.B) {
	tmpDir := b.TempDir()
	cache, err := New(tmpDir)
	if err != nil {
		b.Fatalf("Failed to create cache: %v", err)
	}

	date := "2024-01-15"
	images := map[string]string{"1920x1080": "https://example.com/img.jpg"}
	colors := map[string]string{"highlight": "#ff0000", "primary": "#00ff00", "secondary": "#0000ff"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := cache.Set(date, images, colors)
		if err != nil {
			b.Fatalf("Failed to set cache: %v", err)
		}
	}
}
