package bing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	bingAPIURL  = "https://www.bing.com/HPImageArchive.aspx"
	bingBaseURL = "https://www.bing.com"
)

// Client handles interactions with the Bing wallpaper API
type Client struct {
	httpClient *http.Client
	market     string // e.g., "en-US", "ja-JP"
}

// WallpaperInfo contains metadata about a Bing wallpaper
type WallpaperInfo struct {
	URL           string
	ImageID       string            // Unique image identifier (e.g., "OHR.MartimoaapaFinland_EN-US3685817058")
	ImageURLs     map[string]string // Different size URLs
	Title         string
	Copyright     string
	CopyrightLink string
	Date          string
}

// bingAPIResponse represents the JSON response from Bing's API
type bingAPIResponse struct {
	Images []struct {
		URL          string `json:"url"`
		URLBase      string `json:"urlbase"`
		Title        string `json:"title"`
		Copyright    string `json:"copyright"`
		CopyrightURL string `json:"copyrightlink"`
		StartDate    string `json:"startdate"` // Format: YYYYMMDD
	} `json:"images"`
}

// NewClient creates a new Bing wallpaper client
func NewClient(market string) *Client {
	if market == "" {
		market = "en-US"
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		market: market,
	}
}

// SetLocale updates the market/locale for the client
func (c *Client) SetLocale(locale string) {
	c.market = locale
}

// GetWallpaperInfo fetches metadata for the wallpaper on a given date
// date should be in "YYYY-MM-DD" format
func (c *Client) GetWallpaperInfo(date string) (*WallpaperInfo, error) {
	// Calculate days offset from today
	targetDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	daysAgo := int(today.Sub(targetDate).Hours() / 24)

	if daysAgo < 0 {
		return nil, fmt.Errorf("cannot fetch wallpaper for future dates")
	}

	// Bing API only keeps about 7-8 days of history
	if daysAgo > 7 {
		return nil, fmt.Errorf("wallpaper too old (Bing only keeps ~7 days)")
	}

	// Build API URL
	url := fmt.Sprintf("%s?format=js&idx=%d&n=1&mkt=%s", bingAPIURL, daysAgo, c.market)

	// Make request
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from Bing API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bing API returned status %d", resp.StatusCode)
	}

	// Parse response
	var apiResp bingAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse Bing API response: %w", err)
	}

	if len(apiResp.Images) == 0 {
		return nil, fmt.Errorf("no wallpaper found for date %s", date)
	}

	image := apiResp.Images[0]

	// Construct full URL
	imageURL := bingBaseURL + image.URL
	urlBase := bingBaseURL + image.URLBase

	// Generate different size URLs (based on actual Bing availability)
	imageURLs := map[string]string{
		"UHD":       urlBase + "_UHD.jpg",       // Ultra HD (~3.2MB)
		"1920x1200": urlBase + "_1920x1200.jpg", // Wide (~850KB)
		"1920x1080": urlBase + "_1920x1080.jpg", // Full HD (~320KB)
		"1366x768":  urlBase + "_1366x768.jpg",  // Laptop (~163KB)
		"1280x720":  urlBase + "_1280x720.jpg",  // HD (~179KB)
		"1024x768":  urlBase + "_1024x768.jpg",  // XGA (~66KB)
		"800x600":   urlBase + "_800x600.jpg",   // SVGA (~93KB)
	}

	// Extract image ID from URLBase (e.g., "/th?id=OHR.ImageName_EN-US123456" -> "OHR.ImageName_EN-US123456")
	imageID := extractImageID(image.URLBase)

	return &WallpaperInfo{
		URL:           imageURL,
		ImageID:       imageID,
		ImageURLs:     imageURLs,
		Title:         image.Title,
		Copyright:     image.Copyright,
		CopyrightLink: image.CopyrightURL,
		Date:          date,
	}, nil
}

// extractImageID extracts the image ID from the URLBase
// Example: "/th?id=OHR.MartimoaapaFinland_EN-US3685817058" -> "OHR.MartimoaapaFinland_EN-US3685817058"
func extractImageID(urlBase string) string {
	// URLBase format: "/th?id=<IMAGE_ID>"
	const prefix = "/th?id="
	if len(urlBase) > len(prefix) {
		return urlBase[len(prefix):]
	}
	return urlBase
}

// DownloadWallpaper downloads the actual wallpaper image data
func (c *Client) DownloadWallpaper(info *WallpaperInfo) ([]byte, error) {
	resp, err := c.httpClient.Get(info.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download wallpaper: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wallpaper download returned status %d", resp.StatusCode)
	}

	// Read the entire image into memory
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallpaper data: %w", err)
	}

	return data, nil
}

// GetWallpaper is a convenience method that fetches info and downloads in one call
func (c *Client) GetWallpaper(date string) ([]byte, *WallpaperInfo, error) {
	info, err := c.GetWallpaperInfo(date)
	if err != nil {
		return nil, nil, err
	}

	data, err := c.DownloadWallpaper(info)
	if err != nil {
		return nil, nil, err
	}

	return data, info, nil
}
