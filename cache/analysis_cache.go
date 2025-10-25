package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AnalysisEntry stores AI analysis results for a wallpaper image
type AnalysisEntry struct {
	ImageHash string            `json:"image_hash"`
	Colors    map[string]string `json:"colors"`
}

// AnalysisCache manages AI analysis results cache
type AnalysisCache struct {
	mu          sync.RWMutex
	data        map[string]*AnalysisEntry // key: image_hash
	cacheDir    string
	processing  map[string]*sync.Mutex // Per-image mutexes for concurrent requests
	processingL sync.Mutex             // Protects processing map
}

// NewAnalysisCache creates a new analysis cache
func NewAnalysisCache(cacheDir string) (*AnalysisCache, error) {
	dir := filepath.Join(cacheDir, "analysis")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create analysis cache directory: %w", err)
	}

	return &AnalysisCache{
		data:       make(map[string]*AnalysisEntry),
		cacheDir:   dir,
		processing: make(map[string]*sync.Mutex),
	}, nil
}

// Get retrieves an analysis entry by image hash
func (c *AnalysisCache) Get(imageHash string) *AnalysisEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.data[imageHash]
}

// Set stores an analysis entry and persists to disk
func (c *AnalysisCache) Set(imageHash string, colors map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := &AnalysisEntry{
		ImageHash: imageHash,
		Colors:    colors,
	}

	c.data[imageHash] = entry

	// Persist to disk
	return c.saveToFile(entry)
}

// LoadAll loads all analysis entries from disk
func (c *AnalysisCache) LoadAll() error {
	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read analysis cache directory: %w", err)
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

		var entry AnalysisEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		c.data[entry.ImageHash] = &entry
		loaded++
	}

	if loaded > 0 {
		fmt.Printf("Loaded %d analysis cache entries\n", loaded)
	}

	return nil
}

// saveToFile persists an analysis entry to disk
func (c *AnalysisCache) saveToFile(entry *AnalysisEntry) error {
	// Image hash is already safe for filename (hex string)
	filename := filepath.Join(c.cacheDir, entry.ImageHash+".json")

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal analysis entry: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write analysis cache file: %w", err)
	}

	return nil
}

// GetMutex gets or creates a mutex for a specific image hash
// This ensures only one goroutine processes a given image at a time
func (c *AnalysisCache) GetMutex(imageHash string) *sync.Mutex {
	c.processingL.Lock()
	defer c.processingL.Unlock()

	if mu, exists := c.processing[imageHash]; exists {
		return mu
	}

	mu := &sync.Mutex{}
	c.processing[imageHash] = mu
	return mu
}
