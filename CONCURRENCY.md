# Concurrency Behavior

## Two-Level Cache Architecture

The system uses two independent caches to optimize performance and avoid duplicate AI analysis:

### Level 1: Request Cache
**Storage:** `cache_data/requests/YYYY-MM-DD_locale.json`  
**Key:** `date + locale`  
**Contains:** Metadata about a specific request
- Date
- Locale
- Image ID (extracted from Bing)
- Image URLs (all sizes)

### Level 2: Analysis Cache
**Storage:** `cache_data/analysis/image_hash.json`  
**Key:** `image_hash` (SHA256 hash of image data)  
**Contains:** AI analysis results
- Image Hash (64-character hex string)
- Named colors (highlight, primary, secondary, etc.)

## Why Two Caches?

**Problem:** Bing often serves the same wallpaper image to multiple locales on the same day, but with different metadata IDs.

Example:
- `en-US` gets image ID: `OHR.Example_EN-US123456`
- `ja-JP` gets image ID: `OHR.Example_JA-JP123456`
- But they're the **exact same image!**

**Solution:** Hash the actual image data to detect duplicates.
</text>

<old_text line=30>
**Without shared analysis:**
```
en-US request → Download image A → AI analysis (30s) → Cache
ja-JP request → Download image A → AI analysis (30s) → Cache  ❌ Duplicate work!
```

**With two-level cache:**
```
en-US request → Download metadata → Image ID: "ABC123" → AI analysis (30s) → Cache by image ID
ja-JP request → Download metadata → Image ID: "ABC123" → Cache hit! (instant) ✓
```

**Without shared analysis:**
```
en-US request → Download image A → AI analysis (30s) → Cache
ja-JP request → Download image A → AI analysis (30s) → Cache  ❌ Duplicate work!
```

**With two-level cache:**
```
en-US request → Download metadata → Image ID: "ABC123" → AI analysis (30s) → Cache by image ID
ja-JP request → Download metadata → Image ID: "ABC123" → Cache hit! (instant) ✓
```

## Request Flow

### Fast Path (Full Cache Hit)
```
1. Request: /api/colors?date=2024-01-15&locale=en-US
2. Check request cache → HIT
3. Get image_hash from request entry
4. Check analysis cache by image_hash → HIT
5. Return colors + image URLs (< 1ms)
```

### Image Hash Hit (Analysis exists, different locale)
```
1. Request: /api/colors?date=2024-01-15&locale=ja-JP
2. Check request cache → MISS (never requested ja-JP before)
3. Download wallpaper image
4. Generate image hash
5. Check analysis cache by image_hash → HIT (same image as en-US!)
6. Cache request metadata
7. Return colors + image URLs (~2s, no AI needed)
```

### Cold Start (Full Cache Miss)
```
1. Request: /api/colors?date=2024-01-15&locale=en-US
2. Check request cache → MISS
3. Download wallpaper image (~2s)
4. Generate image hash (SHA256)
5. Check analysis cache by image_hash → MISS
6. Acquire mutex on image_hash
7. Double-check analysis cache (another goroutine might have finished)
8. AI analysis (~5-30s)
9. Cache analysis by image_hash
10. Cache request metadata
11. Release mutex
12. Return colors + image URLs (~7-35s)
```

## Locking Strategy

### Key Insight: Lock on Image Hash, Not Metadata

**Why hash instead of Bing's image ID?**
- Bing returns different IDs for same image across locales
- `OHR.Example_EN-US123` vs `OHR.Example_JA-JP456` → same image!
- Hash detects identical image content

**Locking approach:**
```
Mutex key: image_hash (SHA256 of image data)
Benefit: Only blocks requests analyzing the EXACT SAME image
```

### Example: Parallel Requests

**Scenario 1: Same image, different locales (different Bing IDs)**
```
Time  en-US                        ja-JP
────────────────────────────────────────────────────────────
T0    Download image               
T1    Hash = "abc123..."
T2    Analysis cache MISS
T3    Acquire mutex(abc123) ✓
T4    AI analyzing...              Download image
T5    AI analyzing...              Hash = "abc123..." (SAME!)
T6    AI analyzing...              Analysis cache MISS
T7    AI analyzing...              Try mutex(abc123) → BLOCKED
T10   Cache analysis
T11   Release mutex
T12                                 Mutex acquired!
T13                                 Double-check → CACHE HIT ✓
T14                                 Return (instant)
```

**Scenario 2: Different images (truly different content)**
```
Time  en-US                        ja-JP (different image)
────────────────────────────────────────────────────────────
T0    Download image               Download image
T1    Hash = "abc123..."           Hash = "xyz789..." (DIFFERENT!)
T2    Acquire mutex(abc123) ✓      Acquire mutex(xyz789) ✓
T3    AI analyzing...              AI analyzing...
T4    Both process in parallel (no blocking!) ✓
```

## Concurrency Guarantees

### Per-Image-Hash Mutex
Each unique image hash gets its own mutex:
```go
processing map[string]*sync.Mutex

"abc123...def" → Mutex A  (en-US and ja-JP share this if same image)
"xyz789...ghi" → Mutex B  (different image)
```

### Double-Check Pattern
```go
// Download image and generate hash
imageData := downloadImage()
imageHash := sha256(imageData)

// First check (before lock)
if analysis := analysisCache.Get(imageHash); analysis != nil {
    return analysis  // Fast path
}

// Acquire lock
mutex := analysisCache.GetMutex(imageHash)
mutex.Lock()
defer mutex.Unlock()

// Second check (after lock)
if analysis := analysisCache.Get(imageHash); analysis != nil {
    return analysis  // Another goroutine finished while we waited
}

// Do the expensive work
analyze(imageData)
cache()
```

This ensures only ONE goroutine analyzes each unique image, even with hundreds of concurrent requests.

## Performance Characteristics

### Cache Patterns

| Request Type | Request Cache | Analysis Cache | Time | AI Calls |
|--------------|---------------|----------------|------|----------|
| **Full hit** | HIT | HIT | <1ms | 0 |
| **Hash hit** | MISS | HIT (via hash) | ~2s | 0 |
| **Cold start** | MISS | MISS | ~7-35s | 1 |
| **Concurrent (same image)** | MISS | MISS → HIT | ~7-35s (first), instant (rest) | 1 |

### Real-World Example

```
09:00 - User requests en-US wallpaper
        → Download + AI analysis (30s)
        → Cache by image_id

09:30 - User requests ja-JP wallpaper (same image, different Bing ID!)
        → Download image (2s)
        → Hash matches! Analysis cache HIT
        → No AI call needed ✓

09:45 - User requests de-DE wallpaper (same image, different Bing ID!)
        → Download image (2s)
        → Hash matches! Analysis cache HIT
        → No AI call needed ✓

Result: 3 requests, only 1 AI analysis (30s + 2s + 2s = 34s total vs 90s without optimization)
```

## HTTP Connection Behavior

Each request runs in its own goroutine with an open HTTP connection:

```
Client A (en-US) ──┬── Goroutine 1 → 30s (AI analysis)
Client B (ja-JP) ──┼── Goroutine 2 → 2s  (metadata hit)
Client C (de-DE) ──┴── Goroutine 3 → 2s  (metadata hit)
```

- Goroutine 1 holds mutex on image_id, does analysis
- Goroutine 2 & 3 don't need mutex (cache hit before reaching lock)
- All connections stay open until their goroutine completes

## File Structure

```
cache_data/
├── requests/
│   ├── 2024-01-15_en-US.json  → references hash "abc123...def"
│   ├── 2024-01-15_ja-JP.json  → references hash "abc123...def" (same!)
│   └── 2024-01-15_de-DE.json  → references hash "abc123...def" (same!)
└── analysis/
    └── abc123def456789012345678901234567890123456789012345678901234.json
        ↑ Shared by all above! (SHA256 hash of image data)
```

Request files are small (metadata only). Analysis files are named by image content hash and contain the expensive AI results. Same image = same hash = same file!

## Benefits

1. **Content-based deduplication** - Identical images detected even with different Bing IDs
2. **No duplicate AI analysis** - Same image analyzed once regardless of how many locales request it
3. **Optimal locking** - Only blocks when analyzing the exact same image (by content)
4. **Fast hash hits** - Different locales get instant colors if image content already analyzed
5. **Memory efficient** - Analysis results shared across all requests with same image
6. **Disk persisted** - Both caches survive restarts
7. **Inspectable** - JSON files can be examined/debugged easily
8. **SHA256 security** - Cryptographically secure hash prevents collisions

## Testing

Our test suite verifies:
- ✅ Same image hash blocks concurrent analysis attempts
- ✅ Different image hashes process in parallel
- ✅ Multiple locales can share analysis results via hash matching
- ✅ Image hashing is deterministic and collision-resistant
- ✅ Request cache and analysis cache stay synchronized
- ✅ Double-check pattern prevents race conditions
- ✅ File persistence works across cache restarts