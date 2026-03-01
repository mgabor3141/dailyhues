package bing

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// AllMarkets contains all known Bing wallpaper markets for global wallpaper detection
var AllMarkets = []string{
	"en-US", "en-GB", "en-CA", "en-AU", "en-IN",
	"ja-JP", "zh-CN", "zh-TW", "de-DE", "fr-FR",
	"es-ES", "it-IT", "pt-BR", "ru-RU", "ko-KR",
}

// baseImageNameRegex matches the locale/ROW suffix in Bing image IDs
var baseImageNameRegex = regexp.MustCompile(`_([A-Z]{2}-[A-Z]{2}|ROW)\d+$`)

const (
	bingAPIURL  = "https://www.bing.com/HPImageArchive.aspx"
	bingBaseURL = "https://www.bing.com"
	httpTimeout = 30 * time.Second
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
	StartDate     string // Format: YYYYMMDD (e.g., "20251019")
	FullStartDate string // Format: YYYYMMDDHHMM (e.g., "202510190700")
	EndDate       string // Format: YYYYMMDD (e.g., "20251020")
}

// bingAPIResponse represents the JSON response from Bing's API
type bingAPIResponse struct {
	Images []struct {
		URL           string `json:"url"`
		URLBase       string `json:"urlbase"`
		Title         string `json:"title"`
		Copyright     string `json:"copyright"`
		CopyrightURL  string `json:"copyrightlink"`
		StartDate     string `json:"startdate"`     // Format: YYYYMMDD (e.g., "20251019")
		FullStartDate string `json:"fullstartdate"` // Format: YYYYMMDDHHMM (e.g., "202510190700")
		EndDate       string `json:"enddate"`       // Format: YYYYMMDD (e.g., "20251020")
	} `json:"images"`
}

// NewClient creates a new Bing wallpaper client
func NewClient(market string) *Client {
	if market == "" {
		market = "en-US"
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: httpTimeout,
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
		daysAgo = 0
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
		StartDate:     image.StartDate,
		FullStartDate: image.FullStartDate,
		EndDate:       image.EndDate,
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

// GetWallpaperInfoByDaysAgo fetches metadata for the wallpaper by days ago
// daysAgo should be 0 (today), 1 (yesterday), etc.
func (c *Client) GetWallpaperInfoByDaysAgo(daysAgo int) (*WallpaperInfo, error) {
	// Validate range
	if daysAgo < 0 {
		return nil, fmt.Errorf("daysAgo cannot be negative")
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
		return nil, fmt.Errorf("no wallpaper found for daysAgo=%d", daysAgo)
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
		StartDate:     image.StartDate,
		FullStartDate: image.FullStartDate,
		EndDate:       image.EndDate,
	}, nil
}

// GetWallpaperByDaysAgo is a convenience method that fetches info and downloads by daysAgo
func (c *Client) GetWallpaperByDaysAgo(daysAgo int) ([]byte, *WallpaperInfo, error) {
	info, err := c.GetWallpaperInfoByDaysAgo(daysAgo)
	if err != nil {
		return nil, nil, err
	}

	data, err := c.DownloadWallpaper(info)
	if err != nil {
		return nil, nil, err
	}

	return data, info, nil
}

// ExtractBaseImageName extracts the base image name from a Bing image ID,
// removing the locale-specific suffix (e.g., "_EN-US1234567890" or "_ROW1234567890").
// Example: "OHR.BalearesDay_DE-DE6256697714" -> "OHR.BalearesDay"
func ExtractBaseImageName(imageID string) string {
	return baseImageNameRegex.ReplaceAllString(imageID, "")
}

// FindMostCommonWallpaper queries all given markets concurrently for the wallpaper
// at the given daysAgo offset and returns the most common image's info.
// It prefers metadata from preferredMarket if it has the same image, otherwise
// falls back to any market with a meaningful title (not "Info").
// matchingMarkets indicates which markets had the winning image.
func (c *Client) FindMostCommonWallpaper(daysAgo int, markets []string, preferredMarket string) (imageData []byte, info *WallpaperInfo, matchingMarkets map[string]bool, err error) {
	type result struct {
		market string
		info   *WallpaperInfo
		err    error
	}

	var wg sync.WaitGroup
	ch := make(chan result, len(markets))

	for _, market := range markets {
		wg.Add(1)
		go func(mkt string) {
			defer wg.Done()
			client := &Client{httpClient: c.httpClient, market: mkt}
			info, err := client.GetWallpaperInfoByDaysAgo(daysAgo)
			ch <- result{mkt, info, err}
		}(market)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect results grouped by base image name
	type imageGroup struct {
		count int
		infos map[string]*WallpaperInfo // market -> info
	}
	groups := make(map[string]*imageGroup)

	for r := range ch {
		if r.err != nil {
			slog.Info("Failed to fetch wallpaper from market", "market", r.market, "error", r.err)
			continue
		}
		baseName := ExtractBaseImageName(r.info.ImageID)
		if groups[baseName] == nil {
			groups[baseName] = &imageGroup{infos: make(map[string]*WallpaperInfo)}
		}
		groups[baseName].count++
		groups[baseName].infos[r.market] = r.info
	}

	// Find most common image
	var bestName string
	var bestCount int
	for name, group := range groups {
		if group.count > bestCount {
			bestName = name
			bestCount = group.count
		}
	}

	if bestName == "" {
		return nil, nil, nil, fmt.Errorf("no wallpaper found across any market")
	}

	slog.Info("Found most common wallpaper", "baseName", bestName, "count", bestCount, "totalMarkets", len(markets))

	// Pick the best metadata source from the winning group
	group := groups[bestName]
	var selectedInfo *WallpaperInfo

	// 1. Prefer the user's locale market
	if info, ok := group.infos[preferredMarket]; ok && info.Title != "Info" {
		selectedInfo = info
	}

	// 2. Fall back to any market with a meaningful title
	if selectedInfo == nil {
		for _, info := range group.infos {
			if info.Title != "Info" {
				selectedInfo = info
				break
			}
		}
	}

	// 3. Last resort: any market at all
	if selectedInfo == nil {
		for _, info := range group.infos {
			selectedInfo = info
			break
		}
	}

	if selectedInfo == nil {
		return nil, nil, nil, fmt.Errorf("unexpected: no info in most common group")
	}

	// Build matching markets map
	matchingMarkets = make(map[string]bool, len(group.infos))
	for mkt := range group.infos {
		matchingMarkets[mkt] = true
	}

	// Download the image
	data, err := c.DownloadWallpaper(selectedInfo)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to download global wallpaper: %w", err)
	}

	return data, selectedInfo, matchingMarkets, nil
}
