# Changelog

## 2026-03-01 — v2 API

### Added

- **`GET /api/v2/colors`** — new primary endpoint with `region`, `language`, and `daysAgo` query parameters
- **Global wallpaper mode** (`region=global`) — queries all 15 Bing markets concurrently and picks the most common wallpaper, so results aren't tied to a single locale
- **AI-translated titles and descriptions** — a lightweight text-only AI call generates a clean title and short description in the configured language
- **Instance-level configuration** via `ALLOWED_REGIONS` and `ALLOWED_LANGUAGES` env vars, gating which parameter combinations are accepted (and thus which combinations can trigger AI inference)
- **Translation cache** — translations are cached by image hash + language, separate from the color analysis cache. Adding a new language only triggers a cheap text-only AI call, not a full image re-analysis

### Changed

- **Color analysis is now shared by image hash** — if multiple regions serve the same wallpaper (common with Bing), the expensive image-based AI call runs only once
- **Cache key scheme** — request cache now keys by region + daysAgo instead of locale + daysAgo. Old cache files with the `"locale"` JSON key are automatically migrated on startup

### Deprecated

- **`GET /api/colors`** (v1) — still works, returns the instance's default result (same cache as v2). When the global wallpaper differs from the en-US wallpaper, a deprecation notice is appended to the `copyright` field. The `locale` query parameter is ignored. Migrate to `/api/v2/colors`.
- **`ALLOWED_LOCALES` env var** — replaced by `ALLOWED_REGIONS`
