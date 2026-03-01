# dailyhues

Get an AI-extracted color palette with Bing's wallpaper of the day.

Here's an example of what you can do with the palette:

<img width="2560" height="1440" alt="image" src="https://github.com/user-attachments/assets/27e5fafc-2607-4419-a5e0-151d068fe6f2" />

## Usage

I host a public instance at [dailyhues.up.railway.app](https://dailyhues.up.railway.app).

Use the API to get today's wallpaper and a matching color gradient. The palette is designed to always fit well with the wallpaper, and can be used for styling UI elements.

```sh
curl https://dailyhues.up.railway.app/api/v2/colors
```

The first request of the day takes about 30 seconds (downloads the wallpaper from Bing and asks AI for colors). Subsequent requests are instant (cached until the next hour boundary).

With the response data, you can:
  - Download the wallpaper for your screen size
  - Apply the gradient to a focused window's border
  - Use either color as a highlight for UI elements — `gradient_from` works near the top of the screen (e.g. a bar), `gradient_to` near the bottom

You can find a practical example in my [dotfiles](https://github.com/mgabor3141/dots/blob/main/.local/bin/bing-wallpaper.sh) repository.

### Parameters

```
GET /api/v2/colors?region=global&language=English&daysAgo=0
```

All parameters are optional and default to the instance's configured values.

| Parameter  | Default    | Description |
|------------|------------|-------------|
| `region`   | `global`   | `global` (most common wallpaper across all Bing markets) or a Bing locale code like `en-US`, `ja-JP` |
| `language` | `English`  | Language for the AI-generated title and description |
| `daysAgo`  | `0`        | `0` (today) through `7` (a week ago) — Bing only keeps 7 days of history |

Available regions and languages are configured per instance. The public instance serves `global` region with `English` translations.

### Example Response

```json
{
  "startdate": "20260228",
  "fullstartdate": "202602281600",
  "enddate": "20260301",
  "images": {
    "UHD": "https://www.bing.com/th?id=OHR.BalearesDay_ZH-CN5024902433_UHD.jpg",
    "1920x1200": "https://www.bing.com/th?id=OHR.BalearesDay_ZH-CN5024902433_1920x1200.jpg",
    "1920x1080": "https://www.bing.com/th?id=OHR.BalearesDay_ZH-CN5024902433_1920x1080.jpg",
    "1366x768": "https://www.bing.com/th?id=OHR.BalearesDay_ZH-CN5024902433_1366x768.jpg",
    "1280x720": "https://www.bing.com/th?id=OHR.BalearesDay_ZH-CN5024902433_1280x720.jpg",
    "1024x768": "https://www.bing.com/th?id=OHR.BalearesDay_ZH-CN5024902433_1024x768.jpg",
    "800x600": "https://www.bing.com/th?id=OHR.BalearesDay_ZH-CN5024902433_800x600.jpg"
  },
  "colors": {
    "gradient_angle": 135,
    "gradient_from": "#FF9F6B",
    "gradient_to": "#C2185B"
  },
  "title": "Ancient Stone Steps in Ibiza",
  "description": "Historic stairway in Ibiza, Balearic Islands, Spain",
  "copyright": "伊维萨岛, 巴利阿里群岛, 西班牙 (© tokar/Shutterstock)",
  "copyright_link": "https://www.bing.com/search?q=...",
  "cached_at": "2026-03-01T15:25:09Z"
}
```

### Deprecated: v1 endpoint

The original `/api/colors` endpoint still works but is deprecated. It returns the instance's default result (same data as `/api/v2/colors` with no parameters). When the global wallpaper differs from the en-US wallpaper, a deprecation notice is appended to the `copyright` field.

```sh
# Still works, but please migrate to /api/v2/colors
curl https://dailyhues.up.railway.app/api/colors
```

## Self-Hosting

### Instance Configuration

Configure your instance with environment variables (see [`.env.example`](.env.example)):

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENROUTER_API_KEY` | *(required)* | [OpenRouter](https://openrouter.ai/) API key for AI color analysis and translation |
| `ALLOWED_REGIONS` | `global` | Comma-separated list of regions the API accepts. `global` picks the most common wallpaper across all Bing markets. Locale codes like `en-US`, `ja-JP` fetch that market's wallpaper. First value is the default. |
| `ALLOWED_LANGUAGES` | `English` | Comma-separated list of languages for AI translation. First value is the default. Leave empty to disable translation. |
| `PORT` | `8080` | Server port |
| `CACHE_DIR` | `./cache_data` | Directory for persistent cache files |
| `DEBUG_AI_RESPONSES` | | Set to `true` to write raw AI responses to disk |
| `LOG_FORMAT` | | Set to `json` for structured JSON logging |

**Cost control:** The allowed regions and languages gate which parameter combinations the API accepts. Only configured combinations trigger AI inference — all others are rejected. Color analysis (expensive, uses image input) is cached by image hash and shared across all regions that happen to have the same wallpaper. Translation (cheap, text-only) is cached by image hash + language.

### Docker

```bash
docker build -t dailyhues .
docker run -p 8080:8080 \
  -e OPENROUTER_API_KEY=your_key \
  -e ALLOWED_REGIONS=global \
  -e ALLOWED_LANGUAGES=English \
  dailyhues
```

### Local Development

Uses [devenv.sh](https://devenv.sh/) for development dependencies.

1. Copy `.env.example` to `.env` and add your OpenRouter API key
2. Run the server:
```bash
dev
```
