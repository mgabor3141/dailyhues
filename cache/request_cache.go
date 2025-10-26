package cache

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RequestEntry stores metadata about a wallpaper request
type RequestEntry struct {
	Locale        string            `json:"locale"`
	DaysAgo       int               `json:"days_ago"`
	ImageHash     string            `json:"image_hash"`
	ImageURLs     map[string]string `json:"image_urls"`
	Title         string            `json:"title"`
	Copyright     string            `json:"copyright"`
	CopyrightLink string            `json:"copyright_link"`
	StartDate     string            `json:"startdate"`     // Format: YYYYMMDD (e.g., "20251019")
	FullStartDate string            `json:"fullstartdate"` // Format: YYYYMMDDHHMM (e.g., "202510190700")
	EndDate       string            `json:"enddate"`       // Format: YYYYMMDD (e.g., "20251020")
	ExpiresAt     time.Time         `json:"expires_at"`
}

// RequestCache manages request metadata cache
type RequestCache struct {
	mu       sync.RWMutex
	data     map[string]*RequestEntry // key: "locale_daysago"
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

// makeKey creates a cache key from locale and daysAgo
func (c *RequestCache) makeKey(locale string, daysAgo int) string {
	return fmt.Sprintf("%s_%d", locale, daysAgo)
}

// Get retrieves a request entry
func (c *RequestCache) Get(locale string, daysAgo int) *RequestEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := c.makeKey(locale, daysAgo)
	return c.data[key]
}

// Set stores a request entry and persists to disk
func (c *RequestCache) Set(locale string, daysAgo int, imageHash string, imageURLs map[string]string, title, copyright, copyrightLink, startDate, fullStartDate, endDate string, expiresAt time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &RequestEntry{
		Locale:        locale,
		DaysAgo:       daysAgo,
		ImageHash:     imageHash,
		ImageURLs:     imageURLs,
		Title:         title,
		Copyright:     copyright,
		CopyrightLink: copyrightLink,
		StartDate:     startDate,
		FullStartDate: fullStartDate,
		EndDate:       endDate,
		ExpiresAt:     expiresAt,
	}

	key := c.makeKey(locale, daysAgo)
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

		key := c.makeKey(entry.Locale, entry.DaysAgo)
		c.data[key] = &entry
		loaded++
	}

	if loaded > 0 {
		log.Printf("Loaded %d request cache entries", loaded)
	}

	return nil
}

// saveToFile persists a request entry to disk
func (c *RequestCache) saveToFile(entry *RequestEntry) error {
	filename := filepath.Join(c.cacheDir, fmt.Sprintf("%s_%d.json", entry.Locale, entry.DaysAgo))

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal request entry: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write request cache file: %w", err)
	}

	return nil
}
