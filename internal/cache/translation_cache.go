package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TranslationEntry stores AI-translated title and description for a wallpaper image
type TranslationEntry struct {
	ImageHash   string `json:"image_hash"`
	Language    string `json:"language"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// TranslationCache manages AI translation results cache
type TranslationCache struct {
	mu          sync.RWMutex
	data        map[string]*TranslationEntry // key: "imageHash_language"
	cacheDir    string
	processing  map[string]*sync.Mutex // Per-key mutexes for concurrent requests
	processingL sync.Mutex             // Protects processing map
}

// NewTranslationCache creates a new translation cache
func NewTranslationCache(cacheDir string) (*TranslationCache, error) {
	dir := filepath.Join(cacheDir, "translations")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create translation cache directory: %w", err)
	}

	return &TranslationCache{
		data:       make(map[string]*TranslationEntry),
		cacheDir:   dir,
		processing: make(map[string]*sync.Mutex),
	}, nil
}

// makeKey creates a cache key from image hash and language
func (c *TranslationCache) makeKey(imageHash, language string) string {
	return imageHash + "_" + sanitizeLanguage(language)
}

// sanitizeLanguage normalizes a language string for use as a cache key / filename
func sanitizeLanguage(language string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '_'
	}, language)
}

// Get retrieves a translation entry by image hash and language
func (c *TranslationCache) Get(imageHash, language string) *TranslationEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.data[c.makeKey(imageHash, language)]
}

// Set stores a translation entry and persists to disk
func (c *TranslationCache) Set(imageHash, language, title, description string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &TranslationEntry{
		ImageHash:   imageHash,
		Language:    language,
		Title:       title,
		Description: description,
	}

	c.data[c.makeKey(imageHash, language)] = entry

	// Persist to disk
	return c.saveToFile(entry)
}

// LoadAll loads all translation entries from disk
func (c *TranslationCache) LoadAll() error {
	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read translation cache directory: %w", err)
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

		var entry TranslationEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		c.data[c.makeKey(entry.ImageHash, entry.Language)] = &entry
		loaded++
	}

	if loaded > 0 {
		slog.Info("Loaded translation cache entries", "count", loaded)
	}

	return nil
}

// saveToFile persists a translation entry to disk
func (c *TranslationCache) saveToFile(entry *TranslationEntry) error {
	filename := filepath.Join(c.cacheDir, c.makeKey(entry.ImageHash, entry.Language)+".json")

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal translation entry: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write translation cache file: %w", err)
	}

	return nil
}

// GetMutex gets or creates a mutex for a specific image hash + language combination
func (c *TranslationCache) GetMutex(imageHash, language string) *sync.Mutex {
	c.processingL.Lock()
	defer c.processingL.Unlock()

	key := c.makeKey(imageHash, language)
	if mu, exists := c.processing[key]; exists {
		return mu
	}

	mu := &sync.Mutex{}
	c.processing[key] = mu
	return mu
}

// ReleaseMutex removes the mutex for a specific key from the processing map
func (c *TranslationCache) ReleaseMutex(imageHash, language string) {
	c.processingL.Lock()
	defer c.processingL.Unlock()

	delete(c.processing, c.makeKey(imageHash, language))
}
