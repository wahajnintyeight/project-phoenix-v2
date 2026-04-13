# Size-Based Search Strategy

## Problem
GitHub's Code Search API has a hard limit of 1000 results per query. Previous attempts to use date-based slicing (`created:` or `pushed:` qualifiers) failed because:
1. `CodeResult` struct has no timestamp fields
2. Date qualifiers are NOT supported for code search (only for repository/issue search)
3. Sort options are ignored for code search

## Solution: Size-Based Bisection

The `size:` qualifier IS supported for code search and works on byte ranges. We can recursively bisect the size range to bypass the 1000-result cap.

### Implementation

#### New Methods in `github-client.go`

1. **`SearchCodeAll(query, correlationID)`** - Public method that initiates exhaustive search
   - Searches file sizes from 0 to 384KB (GitHub's max indexed file size)
   - Returns deduplicated results by SHA
   - Automatically handles bisection when hitting the 1000-result cap
   - Respects `MAX_RESULT_CAP` environment variable

2. **`searchSizeRange(query, minBytes, maxBytes, maxResults, currentCount, correlationID)`** - Recursive helper
   - Searches within a specific size range
   - Tracks current result count against max cap
   - If results < 1000 or range is 1 byte: returns results
   - If results = 1000: bisects the range and recursively searches both halves
   - Stops early if `MAX_RESULT_CAP` is reached
   - Combines results from both halves

3. **`getMaxResultCap()`** - Reads `MAX_RESULT_CAP` from environment
   - Returns 0 for unlimited (default)
   - Allows capping results to prevent long-running queries

### Configuration

**Environment Variable: `MAX_RESULT_CAP`**
- Default: `0` (unlimited - fetch all results)
- Example: `MAX_RESULT_CAP=5000` (stop after 5000 results per query)
- Use case: Prevent spending hours on queries with 38k+ results

```bash
# Unlimited (fetch everything)
MAX_RESULT_CAP=0

# Cap at 5000 results per query
MAX_RESULT_CAP=5000

# Cap at 2000 results per query
MAX_RESULT_CAP=2000
```

### How It Works

```
Query: "sk-ant-" with MAX_RESULT_CAP=5000
├─ Search size:0..393216 → 1000 results (hit cap!)
│  ├─ Bisect at 196608 bytes
│  ├─ Search size:0..196608 → 450 results ✓ (total: 450)
│  └─ Search size:196609..393216 → 1000 results (hit cap again!)
│     ├─ Bisect at 294912 bytes
│     ├─ Search size:196609..294912 → 380 results ✓ (total: 830)
│     └─ Search size:294913..393216 → 1000 results (hit cap again!)
│        ├─ Bisect at 327680 bytes
│        ├─ Search size:294913..327680 → 920 results ✓ (total: 1750)
│        └─ Search size:327681..393216 → 1000 results (hit cap again!)
│           ├─ Continue bisecting...
│           └─ Stop when total reaches 5000 ✓
└─ Total: 5000 unique results (after deduplication)
```

### Deduplication

Files with identical content share the same SHA across different repositories. We deduplicate by SHA to avoid processing the same file multiple times when it appears in overlapping size ranges.

```go
seen := make(map[string]struct{})
for _, result := range allResults {
    sha := result.GetSHA()
    if _, exists := seen[sha]; !exists {
        seen[sha] = struct{}{}
        unique = append(unique, result)
    }
}
```

### Usage

The scraper handler uses `SearchCodeAll` which respects the cap:

```go
// Searches with MAX_RESULT_CAP from environment
githubResults, err := h.githubClient.SearchCodeAll(query.QueryPattern, correlationID)
```

### Rate Limiting

The existing rate limiters still apply:
- Search API: 30 requests/minute (burst of 5)
- Content API: 83 requests/minute (burst of 20)

Bisection increases the number of search requests, but the rate limiter ensures we stay within GitHub's limits.

### Benefits

1. **Bypasses 1000-result cap** - Can retrieve more than 1000 results per query
2. **Configurable cap** - Set `MAX_RESULT_CAP` to prevent runaway queries
3. **Uses supported qualifiers** - `size:` works for code search (unlike `created:`)
4. **Automatic bisection** - Recursively splits ranges only when needed
5. **Early termination** - Stops when cap is reached, saving API calls
6. **Deduplication** - Removes duplicate SHAs from overlapping ranges
7. **Respects rate limits** - Works with existing rate limiter infrastructure

### Trade-offs

- **More API calls**: Bisection requires multiple searches instead of one
- **Slower for large result sets**: Each bisection adds latency
- **Rate limit consumption**: Uses more of the 30 req/min search quota

For queries with < 1000 results, behavior is identical to the old method (single search, no bisection).

### Recommended Settings

**For fast cycles with reasonable coverage:**
```bash
MAX_RESULT_CAP=5000
MAX_QUERIES_PER_CYCLE=5
```

**For exhaustive search (slow but complete):**
```bash
MAX_RESULT_CAP=0
MAX_QUERIES_PER_CYCLE=0
```

**For balanced approach:**
```bash
MAX_RESULT_CAP=3000
MAX_QUERIES_PER_CYCLE=10
```
