package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	openRouterURL       = "https://openrouter.ai/api/v1/chat/completions"
	claudeModel         = "anthropic/claude-sonnet-4.5"
	aiRequestTimeout    = 60 * time.Second
	colorAnalysisPrompt = `You are a professional UI/UX designer and artist with a strong background in color theory and accessibility guidelines. You are working on the theme for a desktop window manager, and need to design a gradient for when the attached image is set as the desktop wallpaper. Please design a gradient that will work well as the color for the focused window's border!

- Think about the mood of the image and how you can use the UI colors to enhance it.
- Apply your extensive knowledge of color theory to design the perfect gradient for this wallpaper.
- The most important task is to balance adequate contrast for readability with pleasing colors that are harmonious with the image's color palette.
- The "gradient_from" color must have adequate contrast as background for black text.
- Make sure all parts of the gradient pop against the background, especially at the top and the bottom of the image! Use colors that are at least somewhat vibrant because of this, avoid grays if possible.
- You can choose to use two similar colors for a subtle gradient, or distinct ones if the composition calls for it.
- The gradient direction should compliment the image, but keep in mind that the bottom and top of the image are the most important areas for contrast!
- Keep in mind that the colors should have enough contrast to be readable, but must not clash with the image's colors.

(Reminder: A gradient direction of 135 degrees goes from the top left to the bottom right, 180 degrees goes from top to bottom, etc.)

Reply only with a JSON object with the following format. Do not include any additional text or comments.

{"gradient_from": "#34495e", "gradient_to": "#456789", "gradient_angle": 45}`
)

// Analyzer handles AI-powered color analysis of images
type Analyzer struct {
	apiKey     string
	httpClient *http.Client
}

// NewAnalyzer creates a new AI analyzer
func NewAnalyzer(apiKey string) *Analyzer {
	return &Analyzer{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: aiRequestTimeout,
		},
	}
}

// openRouterRequest represents the request format for OpenRouter API
type openRouterRequest struct {
	Model     string    `json:"model"`
	Reasoning reasoning `json:"reasoning"`
	Messages  []message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

type reasoning struct {
	Enabled bool `json:"enabled"`
}

type message struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

// openRouterResponse represents the response from OpenRouter API
type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// debugResponse contains the full debug information for an AI call
type debugResponse struct {
	Timestamp    string                 `json:"timestamp"`
	ImageHash    string                 `json:"image_hash"`
	ImageName    string                 `json:"image_name"`
	ImageSize    int                    `json:"image_size_bytes"`
	Model        string                 `json:"model"`
	Content      string                 `json:"content"`
	ParsedColors map[string]interface{} `json:"parsed_colors"`
	Usage        map[string]int         `json:"usage,omitempty"`
	RawResponse  *openRouterResponse    `json:"raw_response"`
}

// saveDebugResponse saves the AI response to a debug file
func (a *Analyzer) saveDebugResponse(imageHash string, imageName string, imageSize int, apiResp *openRouterResponse, colors map[string]interface{}) error {
	// Only save debug responses if explicitly enabled
	if os.Getenv("DEBUG_AI_RESPONSES") != "true" {
		return nil
	}

	// Create debug directory if it doesn't exist
	debugDir := "debug_responses"
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return fmt.Errorf("failed to create debug directory: %w", err)
	}

	// Prepare debug data
	timestamp := time.Now().Format("2006-01-02")
	debug := debugResponse{
		Timestamp:    time.Now().Format(time.RFC3339),
		ImageHash:    imageHash,
		ImageName:    imageName,
		ImageSize:    imageSize,
		Model:        claudeModel,
		ParsedColors: colors,
		RawResponse:  apiResp,
	}

	if len(apiResp.Choices) > 0 {
		debug.Content = apiResp.Choices[0].Message.Content
	}

	if apiResp.Usage != nil {
		debug.Usage = map[string]int{
			"prompt_tokens":     apiResp.Usage.PromptTokens,
			"completion_tokens": apiResp.Usage.CompletionTokens,
			"total_tokens":      apiResp.Usage.TotalTokens,
		}
	}

	// Marshal to pretty JSON
	jsonData, err := json.MarshalIndent(debug, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal debug data: %w", err)
	}

	// Write to file with timestamp, image name, and image hash
	// Sanitize image name for filename
	sanitizedName := regexp.MustCompile(`[^a-zA-Z0-9-_]`).ReplaceAllString(imageName, "_")
	if len(sanitizedName) > 50 {
		sanitizedName = sanitizedName[:50]
	}
	filename := filepath.Join(debugDir, fmt.Sprintf("%s_%s_%s.json", timestamp, sanitizedName, imageHash[:12]))
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write debug file: %w", err)
	}

	slog.Info("Debug response saved", "filename", filename)
	return nil
}

// AnalyzeColors sends an image to Claude via OpenRouter for color analysis
// Returns a map of named hex color codes suitable for theming
func (a *Analyzer) AnalyzeColors(imageData []byte, imageHash string, title string, copyright string) (map[string]interface{}, error) {
	// Resize image to reduce token count
	resizedImage, err := a.resizeImage(imageData, 540)
	if err != nil {
		return nil, fmt.Errorf("failed to resize image: %w", err)
	}

	// Encode image as base64
	base64Image := base64.StdEncoding.EncodeToString(resizedImage)

	// Construct the request
	reqBody := openRouterRequest{
		Model: claudeModel,
		Reasoning: reasoning{
			Enabled: true,
		},
		MaxTokens: 4168,
		Messages: []message{
			{
				Role: "user",
				Content: []contentPart{
					{
						Type: "image_url",
						ImageURL: &imageURL{
							URL: "data:image/jpeg;base64," + base64Image,
						},
					},
					{
						Type: "text",
						Text: fmt.Sprintf(colorAnalysisPrompt, title, copyright),
					},
				},
			},
		},
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", openRouterURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/mgabor3141/dailyhues")
	req.Header.Set("X-Title", "dailyhues")

	// Make the request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp openRouterResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if apiResp.Error != nil {
		return nil, fmt.Errorf("OpenRouter API error: %s (code: %s)", apiResp.Error.Message, apiResp.Error.Code)
	}

	// Extract content from response
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI model")
	}

	content := apiResp.Choices[0].Message.Content

	// Parse the color array from the response
	colors, err := a.parseColorsFromResponse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse colors: %w", err)
	}

	// Save debug response (log error but don't fail the request)
	if debugErr := a.saveDebugResponse(imageHash, title, len(imageData), &apiResp, colors); debugErr != nil {
		slog.Error("Warning: Failed to save debug response", "error", debugErr)
	}

	return colors, nil
}

// parseColorsFromResponse extracts named color codes and other values from the AI's response
// Returns a map with flexible value types to handle both strings (colors) and ints (angles) or other future types
func (a *Analyzer) parseColorsFromResponse(content string) (map[string]interface{}, error) {
	// Try to parse as JSON object first
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		return result, nil
	}

	// If that fails, try to extract JSON object from the text
	// Use a more flexible regex that can handle nested structures and multiline JSON
	jsonObjectRegex := regexp.MustCompile(`(?s)\{[^{}]*(?:\{[^{}]*\}[^{}]*)*\}`)
	matches := jsonObjectRegex.FindStringSubmatch(content)
	if len(matches) > 0 {
		if err := json.Unmarshal([]byte(matches[0]), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("could not extract colors from response: %s", content)
}

// resizeImage resizes an image to a maximum height while maintaining aspect ratio
func (a *Analyzer) resizeImage(imageData []byte, maxHeight int) ([]byte, error) {
	// Decode image
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// If already smaller than max height, return original
	if height <= maxHeight {
		return imageData, nil
	}

	// Calculate new dimensions maintaining aspect ratio
	newHeight := maxHeight
	newWidth := (width * maxHeight) / height

	// Create new image with calculated dimensions
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Simple nearest-neighbor scaling
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := (x * width) / newWidth
			srcY := (y * height) / newHeight
			resized.Set(x, y, img.At(srcX, srcY))
		}
	}

	// Encode to JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("failed to encode resized image: %w", err)
	}

	return buf.Bytes(), nil
}
