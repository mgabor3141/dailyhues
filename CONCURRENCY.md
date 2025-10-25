# Concurrency Behavior

## How Requests Are Handled

### Cache Hit (Instant Response)
```
Request → Check Cache → HIT → Return (< 1ms)
```

HTTP connection stays open for < 1ms, returns immediately.

### Cache Miss (First Request)

```
Request → Check Cache → MISS → Acquire Lock → Download → AI Analysis → Cache → Return
                                      ↓
                                  (7-35s)
```

**Important:** The HTTP connection stays open the entire time. The client waits for the complete response.

### Concurrent Requests (Same Date)

When multiple requests come in for the same date+locale:

```
Time    Request A (en-US)        Request B (en-US)        Request C (ja-JP)
────────────────────────────────────────────────────────────────────────────────────
T0      Cache MISS
T1      Acquire lock ✓
T2      Downloading...           Cache MISS              Cache MISS
T3      Downloading...           Try lock (BLOCKED)      Acquire lock ✓ (different key)
T4      AI analyzing...          (waiting...)            Downloading...
T5      AI analyzing...          (waiting...)            AI analyzing...
T10     Cache & unlock
T11                              Lock acquired ✓
T12                              Cache HIT!
T13                              Return instantly                       Cache & unlock
```

**Key Points:**

1. **Request A**: Does all the work, connection open for ~10s
2. **Request B**: Same date+locale, blocks on mutex, waits for A to finish, then gets cached result
3. **Request C**: Different locale (or date) = different mutex = no blocking

## Per-Date+Locale Locking

Each date+locale combination gets its own mutex:

```go
processing map[string]*sync.Mutex

"2024-01-15_en-US" → Mutex A
"2024-01-15_ja-JP" → Mutex B
"2024-01-16_en-US" → Mutex C
```

Requests for different dates OR different locales never block each other.

## Double-Check Pattern

```go
cacheKey := getCacheKey(date, locale)

// First check (before lock)
if cached := cache.Get(cacheKey); cached != nil {
    return cached  // Fast path
}

// Acquire lock
lock(cacheKey)
defer unlock(cacheKey)

// Second check (after lock)
if cached := cache.Get(cacheKey); cached != nil {
    return cached  // Another request finished while we waited
}

// Do the work
download()
analyze()
cache()
```

This ensures only one request does the expensive work, even with concurrent requests.

## Test Results

**Same date+locale, concurrent requests:**
- First request: ~10 seconds (does the work)
- Second request: ~10 seconds (waits) + instant (cached result)
- Result: Only 1 API call made ✓

**Same date, different locales:**
- Both requests: ~10 seconds each (parallel)
- No blocking between them ✓
- Result: 2 API calls (different wallpapers) ✓

**Different dates:**
- Both requests: ~10 seconds each (parallel)
- No blocking between them ✓
- Result: 2 API calls (expected) ✓

**Cache hits:**
- Always instant (< 1ms)
- Never blocked ✓

## HTTP Connection Behavior

Go's `net/http` server handles each request in its own goroutine:

```
Client A ─┬─ Goroutine 1 → blocks on mutex → waits
Client B ─┼─ Goroutine 2 → returns cached instantly
Client C ─┴─ Goroutine 3 → downloads different date
```

All connections stay alive until their respective goroutines complete.

## Why This Works

1. **No duplicate work**: Mutex prevents multiple AI API calls for same date+locale
2. **No blocking unrelated requests**: Each date+locale combination has its own mutex
3. **Locale isolation**: Different locales for same date fetch different wallpapers in parallel
4. **Client compatibility**: Standard HTTP - clients just wait for response
5. **Memory efficient**: Only active date+locale combinations have mutexes in memory