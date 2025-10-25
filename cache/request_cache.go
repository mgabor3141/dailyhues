package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// RequestEntry stores metadata about a wallpaper request
type RequestEntry struct {
	Date          string            `json:"date"`
	Locale        string            `json:"locale"`
	ImageHash     string            `json:"image_hash"`
	ImageURLs     map[string]string `json:"image_urls"`
	Copyright     string            `json:"copyright"`
	CopyrightLink string            `json:"copyright_link"`
}

// RequestCache manages request metadata cache
type RequestCache struct {
	mu       sync.RWMutex
	data     map[string]*RequestEntry // key: "date_locale"
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

// makeKey creates a cache key from date and locale
func (c *RequestCache) makeKey(date, locale string) string {
	return date + "_" + locale
}

// Get retrieves a request entry
func (c *RequestCache) Get(date, locale string) *RequestEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.makeKey(date, locale)
	return c.data[key]
}

// Set stores a request entry and persists to disk
func (c *RequestCache) Set(date, locale, imageHash string, imageURLs map[string]string, copyright, copyrightLink string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &RequestEntry{
		Date:          date,
		Locale:        locale,
		ImageHash:     imageHash,
		ImageURLs:     imageURLs,
		Copyright:     copyright,
		CopyrightLink: copyrightLink,
	}

	key := c.makeKey(date, locale)
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

		key := c.makeKey(entry.Date, entry.Locale)
		c.data[key] = &entry
		loaded++
	}

	if loaded > 0 {
		fmt.Printf("Loaded %d request cache entries\n", loaded)
	}

	return nil
}

// saveToFile persists a request entry to disk
func (c *RequestCache) saveToFile(entry *RequestEntry) error {
	filename := filepath.Join(c.cacheDir, fmt.Sprintf("%s_%s.json", entry.Date, entry.Locale))

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal request entry: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write request cache file: %w", err)
	}

	return nil
}
