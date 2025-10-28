# dailyhues

Get an AI-extracted color palette with Bing's wallpaper of the day.

Here's an example of what you can do with the palette:

<img width="2560" height="1440" alt="image" src="https://github.com/user-attachments/assets/27e5fafc-2607-4419-a5e0-151d068fe6f2" />

## Usage

I host a public instance at [dailyhues.mgabor.hu](https://dailyhues.mgabor.hu) (for the `en-US` locale).

Use the API to get today's wallpaper and a matching color gradient. The palette is designed to always fit well with the wallpaper, and can be used for styling UI elements.

```sh
curl https://dailyhues.mgabor.hu/api/colors
```

Requests take about 30 seconds if noone has requested the wallpaper today (downloads wallpaper and asks AI for colors). Subsequent requests are instant (cached).

With the response data, you can:
  - Download the wallpaper for your screen size
  - Apply the gradient itself to the focused window's border
  - Apply any of the two colors as a highlight color for UI elements. `gradient_from` is meant to be used near the top of the screen (e.g. waybar), and `gradient_to` near the bottom

You can find a practical example for how I achieved this in my [dotfiles](https://github.com/mgabor3141/dots/blob/main/.local/bin/bing-wallpaper.sh) repository.

### Parameters

```sh
curl https://dailyhues.mgabor.hu/api/colors?locale=en-US&daysAgo=0
```

Both parameters are optional. `daysAgo` defaults to `0` (today), `locale` defaults to `en-US`.

`daysAgo` can be `0` (today), `1` (yesterday), up to `7` (7 days ago). Bing only keeps wallpapers for the last 7 days.

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

## Running Locally

Locales available from Bing: `en-US`, `en-GB`, `en-CA`, `en-AU`, `en-IN`, `ja-JP`, `zh-CN`, `zh-TW`, `de-DE`, `fr-FR`, `es-ES`, `it-IT`, `pt-BR`, `ru-RU`, `ko-KR`

### Docker

Build and run image

```bash
docker build -t dailyhues .
docker run -p 8080:8080 -e OPENROUTER_API_KEY=your_key dailyhues
```

### Local

Use [devenv.sh](https://devenv.sh/) to install development dependencies automatically and run the binary

1. Add your API key to `.env` file:
```bash
# Copy .env.example and add your OpenRouter API key
OPENROUTER_API_KEY=your_actual_api_key_here
```

2. Run the server:
```bash
dev
```
