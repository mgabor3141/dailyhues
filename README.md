# dailyhues

Get an AI-extracted color palette with Bing's wallpaper of the day.

## API

**GET** `/api/colors`
**GET** `/api/colors?locale=en-US&daysAgo=0`

Both parameters are optional. `daysAgo` defaults to `0` (today), `locale` defaults to `en-US`.

`daysAgo` can be `0` (today), `1` (yesterday), up to `7` (7 days ago). Bing only keeps wallpapers for the last 7 days.

**Allowed locales:** `en-US`, `en-GB`, `en-CA`, `en-AU`, `en-IN`, `ja-JP`, `zh-CN`, `zh-TW`, `de-DE`, `fr-FR`, `es-ES`, `it-IT`, `pt-BR`, `ru-RU`, `ko-KR`

### Example Response

```json
{
  "startdate": "20251019",
  "fullstartdate": "202510190700",
  "enddate": "20251020",
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
    "gradient_angle": 135,
    "gradient_from": "#c67d3a",
    "gradient_to": "#6b8d7d",
  },
  "title": "Finland's living peatland",
  "copyright": "Aerial view of peatland in Martimoaapa Mire Reserve, Finland (© romikatarina/Shutterstock)",
  "copyright_link": "https://www.bing.com/search?q=Martimoaapa+Finland&form=hpcapt",
  "cached_at": "2024-01-15T10:30:00Z"
}
```

First request for a wallpaper takes 5-30 seconds (downloads wallpaper + AI analysis). Subsequent requests are instant (cached). Bing is queried for new images every hour.

## Running Locally

### Docker

Build and run locally:
```bash
docker build -t dailyhues .
docker run -p 8080:8080 -e OPENROUTER_API_KEY=your_key dailyhues
```

### Local

Use [devenv.sh](https://devenv.sh/) to install dependencies and run the server

1. **Add your API key to `.env` file:**
```bash
# Copy .env.example and add your OpenRouter API key
OPENROUTER_API_KEY=your_actual_api_key_here
```

2. **Run the server:**
```bash
dev
```
