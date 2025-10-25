package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ColorTheme represents cached color data for a wallpaper
type ColorTheme struct {
	Date      string            `json:"date"`
	Images    map[string]string `json:"images"`
	Colors    map[string]string `json:"colors"`
	CachedAt  time.Time         `json:"cached_at"`
	FromCache bool              `json:"-"` // Don't persist this field
}

// Cache manages color theme caching with thread-safe operations
type Cache struct {
	mu       sync.Mutex
	data     map[string]*ColorTheme
	cacheDir string
}

// New creates a new cache instance
// cacheDir is where cache files will be stored (e.g., "./cache_data")
func New(cacheDir string) (*Cache, error) {
	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	c := &Cache{
		data:     make(map[string]*ColorTheme),
		cacheDir: cacheDir,
	}

	// Clean up old cache files (optional - keeps directory tidy)
	go c.cleanupOldFiles()

	return c, nil
}

// Get retrieves a cached color theme for the given date
// Returns nil if not found in memory or on disk
func (c *Cache) Get(date string) (*ColorTheme, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check in-memory cache first
	if theme, exists := c.data[date]; exists {
		theme.FromCache = true
		return theme, nil
	}

	// Try to load from file
	theme, err := c.loadFromFile(date)
	if err != nil {
		return nil, err
	}

	if theme != nil {
		// Store in memory for faster subsequent access
		c.data[date] = theme
		theme.FromCache = true
		return theme, nil
	}

	return nil, nil
}

// Set stores a color theme in both memory and disk
func (c *Cache) Set(date string, images map[string]string, colors map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	theme := &ColorTheme{
		Date:     date,
		Images:   images,
		Colors:   colors,
		CachedAt: time.Now(),
	}

	// Store in memory
	c.data[date] = theme

	// Persist to disk
	return c.saveToFile(theme)
}

// LoadAll loads all existing cache files from disk into memory
func (c *Cache) LoadAll() error {
	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Cache directory doesn't exist yet, not an error
		}
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	loaded := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only load cache_*.json files
		if !strings.HasPrefix(file.Name(), "cache_") || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		// Extract date from filename (cache_YYYY-MM-DD.json)
		date := file.Name()[6 : len(file.Name())-5]

		// Load the file
		data, err := os.ReadFile(filepath.Join(c.cacheDir, file.Name()))
		if err != nil {
			continue // Skip files we can't read
		}

		var theme ColorTheme
		if err := json.Unmarshal(data, &theme); err != nil {
			continue // Skip files we can't parse
		}

		// Store in memory
		c.data[date] = &theme
		loaded++
	}

	if loaded > 0 {
		fmt.Printf("Loaded %d cache entries from disk\n", loaded)
	}

	return nil
}

// loadFromFile attempts to load a color theme from disk
func (c *Cache) loadFromFile(date string) (*ColorTheme, error) {
	filename := c.getFilename(date)

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File doesn't exist, not an error
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var theme ColorTheme
	if err := json.Unmarshal(data, &theme); err != nil {
		return nil, fmt.Errorf("failed to parse cache file: %w", err)
	}

	return &theme, nil
}

// saveToFile persists a color theme to disk
func (c *Cache) saveToFile(theme *ColorTheme) error {
	filename := c.getFilename(theme.Date)

	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// getFilename generates the cache filename for a given date
func (c *Cache) getFilename(date string) string {
	return filepath.Join(c.cacheDir, fmt.Sprintf("cache_%s.json", date))
}

// cleanupOldFiles removes cache files older than 7 days
// This runs in the background to keep the cache directory tidy
func (c *Cache) cleanupOldFiles() {
	cutoff := time.Now().AddDate(0, 0, -7)

	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(c.cacheDir, file.Name()))
		}
	}
}
