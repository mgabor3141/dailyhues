package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RequestEntry stores metadata about a wallpaper request
type RequestEntry struct {
	Region        string            `json:"region"`
	DaysAgo       int               `json:"days_ago"`
	ImageHash     string            `json:"image_hash"`
	ImageURLs     map[string]string `json:"image_urls"`
	Title         string            `json:"title"`
	Copyright     string            `json:"copyright"`
	CopyrightLink string            `json:"copyright_link"`
	StartDate     string            `json:"startdate"`     // Format: YYYYMMDD (e.g., "20251019")
	FullStartDate string            `json:"fullstartdate"` // Format: YYYYMMDDHHMM (e.g., "202510190700")
	EndDate       string            `json:"enddate"`       // Format: YYYYMMDD (e.g., "20251020")
	EnUSMatch     bool              `json:"en_us_match"`   // whether en-US market had this image (global mode only)
	ExpiresAt     time.Time         `json:"expires_at"`
}

// RequestCache manages request metadata cache
type RequestCache struct {
	mu       sync.RWMutex
	data     map[string]*RequestEntry // key: "region_daysago"
	cacheDir string
}

// NewRequestCache creates a new request cache
func NewRequestCache(cacheDir string) (*RequestCache, error) {
	dir := filepath.Join(cacheDir, "requests")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create request cache directory: %w", err)
	}

	return &RequestCache{
		data:     make(map[string]*RequestEntry),
		cacheDir: dir,
	}, nil
}

// makeKey creates a cache key from region and daysAgo
func (c *RequestCache) makeKey(region string, daysAgo int) string {
	return fmt.Sprintf("%s_%d", region, daysAgo)
}

// Get retrieves a request entry
func (c *RequestCache) Get(region string, daysAgo int) *RequestEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.makeKey(region, daysAgo)
	return c.data[key]
}

// Set stores a request entry and persists to disk
func (c *RequestCache) Set(entry *RequestEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.makeKey(entry.Region, entry.DaysAgo)
	c.data[key] = entry

	// Persist to disk
	return c.saveToFile(entry)
}

// LoadAll loads all request entries from disk
func (c *RequestCache) LoadAll() error {
	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read request cache directory: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	loaded := 0
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(c.cacheDir, file.Name()))
		if err != nil {
			continue
		}

		var entry RequestEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		key := c.makeKey(entry.Region, entry.DaysAgo)
		c.data[key] = &entry
		loaded++
	}

	if loaded > 0 {
		slog.Info("Loaded request cache entries", "count", loaded)
	}

	return nil
}

// sanitizeRegion normalizes a region string for use in cache keys and filenames.
// Allows only alphanumeric characters and hyphens; everything else becomes '_'.
func sanitizeRegion(region string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '_'
	}, region)
}

// saveToFile persists a request entry to disk
func (c *RequestCache) saveToFile(entry *RequestEntry) error {
	filename := filepath.Join(c.cacheDir, fmt.Sprintf("%s_%d.json", sanitizeRegion(entry.Region), entry.DaysAgo))

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal request entry: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write request cache file: %w", err)
	}

	return nil
}
