# Wallpaper Highlight

Get AI-extracted color palettes from Bing's daily wallpapers.

## Setup

```bash
export OPENROUTER_API_KEY="your-key-here"
go run main.go
```

## API

**GET** `/api/colors?date=YYYY-MM-DD&locale=en-US`

Both parameters are optional. `date` defaults to today, `locale` defaults to `en-US`.

**Allowed locales:** `en-US`, `en-GB`, `en-CA`, `en-AU`, `en-IN`, `ja-JP`, `zh-CN`, `zh-TW`, `de-DE`, `fr-FR`, `es-ES`, `it-IT`, `pt-BR`, `ru-RU`, `ko-KR`

### Example Response

```json
{
  "date": "2024-01-15",
  "images": {
    "UHD": "https://www.bing.com/th?id=OHR.Example_UHD.jpg",
    "1920x1200": "https://www.bing.com/th?id=OHR.Example_1920x1200.jpg",
    "1920x1080": "https://www.bing.com/th?id=OHR.Example_1920x1080.jpg",
    "1366x768": "https://www.bing.com/th?id=OHR.Example_1366x768.jpg",
    "1280x720": "https://www.bing.com/th?id=OHR.Example_1280x720.jpg",
    "1024x768": "https://www.bing.com/th?id=OHR.Example_1024x768.jpg",
    "800x600": "https://www.bing.com/th?id=OHR.Example_800x600.jpg"
  },
  "colors": {
    "highlight": "#1a73e8",
    "primary": "#34a853",
    "secondary": "#fbbc04",
    "background": "#ffffff",
    "surface": "#f5f5f5",
    "text": "#212121"
  },
  "cached_at": "2024-01-15T10:30:00Z",
  "from_cache": true
}
```

First request for a date takes 5-30 seconds (downloads wallpaper + AI analysis). Subsequent requests are instant (cached).