# API Key Scraper Service

The API Key Scraper Service is a microservice that discovers and validates exposed API keys on GitHub. It integrates into the project-phoenix-v2 architecture to identify security vulnerabilities by finding leaked credentials for OpenAI, Anthropic, Google AI, and OpenRouter services.

## Table of Contents

- [Architecture](#architecture)
- [Components](#components)
- [API Endpoints](#api-endpoints)
- [Environment Variables](#environment-variables)
- [Deployment](#deployment)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

## Architecture

### Service Overview

The system consists of two main components:

1. **Scraper Service** (Port 8888): Searches GitHub for exposed API keys using the GitHub Code Search API
2. **Verifier Handler** (Worker Service): Validates discovered keys against provider APIs

### Component Interactions

```
┌─────────────────────────────────────────────────────────────────┐
│                         API Gateway (Port 8080)                  │
│  Routes: /api/v1/keys, /api/v1/keys/valid, /api/v1/stats       │
│          /api/v1/config/queries                                  │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         │ HTTP
                         │
        ┌────────────────┴────────────────┐
        │                                  │
        ▼                                  ▼
┌───────────────────┐            ┌──────────────────┐
│ Scraper Service   │            │ Worker Service   │
│   (Port 8888)     │            │   (Port 8082)    │
│                   │            │                  │
│ - Scraper Handler │            │ - Verifier       │
│ - GitHub Client   │            │   Handler        │
│ - Rate Limiter    │            │                  │
└─────┬─────────────┘            └────────┬─────────┘
      │                                   │
      │ RabbitMQ                          │
      │ keys.discovered ──────────────────┤
      │                                   │
      │ keys.validated ◄──────────────────┘
      │
      ▼
┌─────────────────────────────────────────────────┐
│              MongoDB (Port 27017)                │
│  Collections: api_keys, repo_references,        │
│               search_queries                     │
└─────────────────────────────────────────────────┘
```

### Data Flow

1. **Discovery Flow**:
   - Scraper Service executes enabled search queries every 20 minutes
   - GitHub API returns code search results
   - Keys are extracted using provider-specific regex patterns
   - Discovered keys are stored in MongoDB with deduplication
   - RabbitMQ message published to `keys.discovered` topic

2. **Validation Flow**:
   - Verifier Handler processes keys every hour or via RabbitMQ messages
   - Keys with "Pending" status are validated against provider APIs
   - Status updated to Valid, Invalid, ValidNoCredits, or Error
   - RabbitMQ message published to `keys.validated` topic
   - Oldest valid keys deleted when count exceeds 50

## Components

### Scraper Service

**Location**: `pkg/service/scraper-service/`

**Responsibilities**:
- Execute GitHub code searches using configured query patterns
- Extract API keys from search results using regex patterns
- Store discovered keys in MongoDB with deduplication
- Publish discovery events to RabbitMQ
- Enforce GitHub API rate limits (5-second delay between requests)
- Retry failed requests with exponential backoff

**Key Files**:
- `scraper-service.go`: Main service initialization and HTTP server
- `handlers/scraper-handler.go`: Scraping orchestration and key extraction
- `handlers/github-client.go`: GitHub API client with rate limiting
- `handlers/rate-limiter.go`: Rate limit enforcement

### Verifier Handler

**Location**: `pkg/service/worker-service/handlers/verifier-handler.go`

**Responsibilities**:
- Validate API keys against provider endpoints
- Update key status in MongoDB
- Publish validation results to RabbitMQ
- Enforce 50-key limit for valid keys
- Retry failed validations with fixed delay

**Provider Endpoints**:
- **OpenAI**: `https://api.openai.com/v1/models`
- **Anthropic**: `https://api.anthropic.com/v1/messages`
- **Google AI**: `https://generativelanguage.googleapis.com/v1beta/models`
- **OpenRouter**: `https://openrouter.ai/api/v1/models`

### Controllers

**Location**: `internal/controllers/`

**APIKeyController** (`api-key-controller.go`):
- CRUD operations for API keys
- Pagination and filtering
- Statistics aggregation
- Deduplication via unique indexes

**ScraperConfigController** (`scraper-config-controller.go`):
- Manage search query configurations
- Seed default queries on first run
- Track query statistics

### Data Models

**Location**: `internal/model/`

**APIKey** (`api-key.go`):
```go
type APIKey struct {
    ID          primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
    KeyValue    string               `bson:"key_value" json:"key_value"`
    Provider    string               `bson:"provider" json:"provider"`
    Status      string               `bson:"status" json:"status"`
    CreatedAt   time.Time            `bson:"created_at" json:"created_at"`
    ValidatedAt *time.Time           `bson:"validated_at,omitempty" json:"validated_at,omitempty"`
    LastSeenAt  time.Time            `bson:"last_seen_at" json:"last_seen_at"`
    ErrorCount  int                  `bson:"error_count" json:"error_count"`
    RepoRefs    []primitive.ObjectID `bson:"repo_refs" json:"repo_refs"`
}
```

**Status Values**: `Pending`, `Valid`, `ValidNoCredits`, `Invalid`, `Error`

**Provider Values**: `OpenAI`, `Anthropic`, `Google`, `OpenRouter`

## API Endpoints

All endpoints are exposed through the API Gateway on port 8080 with prefix `/api/v1`.

### 1. List API Keys

**Endpoint**: `GET /api/v1/keys`

**Authentication**: Required (session middleware)

**Query Parameters**:
- `page` (int, optional): Page number for pagination (default: 1)
- `provider` (string, optional): Filter by provider
- `status` (string, optional): Filter by status

**Response**:
```json
{
  "code": 1000,
  "message": "Keys retrieved successfully",
  "data": {
    "total_pages": 5,
    "current_page": 1,
    "keys": [
      {
        "id": "507f1f77bcf86cd799439011",
        "key_value": "sk-***************************",
        "provider": "OpenAI",
        "status": "Valid",
        "created_at": "2024-01-15T10:30:00Z",
        "validated_at": "2024-01-15T11:00:00Z",
        "last_seen_at": "2024-01-15T10:30:00Z",
        "error_count": 0
      }
    ]
  }
}
```

### 2. List Valid Keys

**Endpoint**: `GET /api/v1/keys/valid`

**Authentication**: Required

**Response**: Returns only keys with status "Valid"

### 3. Get Statistics

**Endpoint**: `GET /api/v1/stats`

**Authentication**: Required

**Response**:
```json
{
  "code": 1000,
  "message": "Statistics retrieved successfully",
  "data": {
    "total_keys": 150,
    "valid_keys": 45,
    "invalid_keys": 80,
    "pending_keys": 20,
    "error_keys": 5,
    "by_provider": {
      "OpenAI": 60,
      "Anthropic": 40,
      "Google": 30,
      "OpenRouter": 20
    }
  }
}
```

### 4. List Search Queries

**Endpoint**: `GET /api/v1/config/queries`

**Authentication**: Required

**Response**: Returns all configured search queries

### 5. Create Search Query

**Endpoint**: `POST /api/v1/config/queries`

**Authentication**: Required

**Request Body**:
```json
{
  "query_pattern": "\"ANTHROPIC_API_KEY\" extension:js",
  "provider": "Anthropic",
  "enabled": true
}
```

### 6. Delete Search Query

**Endpoint**: `DELETE /api/v1/config/queries/{id}`

**Authentication**: Required

### 7. Health Check

**Endpoint**: `GET /health` (Port 8888)

**Authentication**: Not required

**Response**:
```json
{
  "status": "healthy",
  "service": "scraper-service",
  "time": "2024-01-15T14:30:00Z",
  "mongodb": "connected",
  "rabbitmq": "connected"
}
```

### 8. Metrics

**Endpoint**: `GET /metrics` (Port 8888)

**Authentication**: Not required

**Response**:
```json
{
  "service": "scraper-service",
  "uptime": "2h30m15s",
  "scraping_cycles": 7,
  "keys_discovered": 45,
  "duplicates_found": 12,
  "errors": 2,
  "github_rate_limit": {
    "remaining": 25,
    "reset_at": "2024-01-15T15:00:00Z"
  },
  "last_scrape": "2024-01-15T14:20:00Z"
}
```

## Environment Variables

### Required Variables

```bash
# MongoDB Configuration
MONGO_URI=mongodb://localhost:27017
MONGO_DB_NAME=phoenix_db

# RabbitMQ Configuration
RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# GitHub API Configuration
GITHUB_API_TOKEN=ghp_token1,ghp_token2,ghp_token3

# Service Ports
SCRAPER_SERVICE_PORT=8888
WORKER_SERVICE_PORT=8082
```

### Optional Variables (with defaults)

```bash
# Scraping Configuration
SCRAPING_INTERVAL_MINUTES=20
VALIDATION_INTERVAL_MINUTES=60
GITHUB_RATE_LIMIT_DELAY_SECONDS=5
MAX_VALID_KEYS=50

# Logging Configuration
LOG_LEVEL=INFO
```

### Configuration Validation

The service validates all configuration on startup:
- MongoDB URI and RabbitMQ URL must be valid URLs
- GitHub tokens must be provided (comma-separated for multiple tokens)
- Ports must be numeric
- Intervals and delays must be positive integers

If validation fails, the service exits with a non-zero status code and logs the error.

## Deployment

### Docker Compose

The service is deployed using Docker Compose alongside other project-phoenix-v2 services.

**docker-compose.yml**:
```yaml
services:
  scraper-service:
    image: ghcr.io/wahajnintyeight/scraper-service:latest
    container_name: ppv2-scraper-service
    ports:
      - "8888:8888"
    env_file:
      - .env
    environment:
      - MONGO_URI=${MONGO_URI}
      - MONGO_DB_NAME=${MONGO_DB_NAME}
      - RABBITMQ_URL=${RABBITMQ_URL}
      - GITHUB_API_TOKEN=${GITHUB_API_TOKEN}
      - SCRAPER_SERVICE_PORT=8888
      - SCRAPING_INTERVAL_MINUTES=20
      - VALIDATION_INTERVAL_MINUTES=60
    networks:
      - ppv2-net
    depends_on:
      - mongodb
      - rabbitmq
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8888/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

### Deployment Steps

1. **Build Docker Image**:
```bash
cd project-phoenix-v2/pkg/service/scraper-service
docker build -t scraper-service:latest .
```

2. **Tag and Push to Registry**:
```bash
docker tag scraper-service:latest ghcr.io/wahajnintyeight/scraper-service:latest
docker push ghcr.io/wahajnintyeight/scraper-service:latest
```

3. **Deploy with Docker Compose**:
```bash
docker-compose pull
docker-compose up -d scraper-service
```

4. **Verify Deployment**:
```bash
# Check health
curl http://localhost:8888/health

# Check logs
docker-compose logs -f scraper-service

# Check metrics
curl http://localhost:8888/metrics
```

### Default Queries

On first startup, the service seeds 4 default search queries:

1. **OpenAI**: `"sk-" "openai" extension:env`
2. **Anthropic**: `"sk-ant-" extension:env`
3. **Google**: `"AIza" "google" extension:env`
4. **OpenRouter**: `"sk-or-" extension:env`

These queries search for API keys in `.env` files on GitHub.

## Monitoring

### Health Checks

The `/health` endpoint returns:
- **200 OK**: Service is healthy, MongoDB and RabbitMQ are connected
- **503 Service Unavailable**: MongoDB or RabbitMQ is disconnected

Docker Compose health checks run every 30 seconds and restart the container after 3 consecutive failures.

### Metrics

The `/metrics` endpoint provides real-time statistics:
- **Uptime**: How long the service has been running
- **Scraping Cycles**: Number of completed scraping cycles
- **Keys Discovered**: Total keys found (including duplicates)
- **Duplicates Found**: Keys that already existed in the database
- **Errors**: Number of errors encountered
- **GitHub Rate Limit**: Remaining API quota and reset time
- **Last Scrape**: Timestamp of last scraping cycle

### Logs

The service uses structured logging with correlation IDs for request tracing:

```
2024-01-15T14:30:00Z [INFO] [scraper-service] [RunScrapingCycle] [abc123-def456] Starting scraping cycle
2024-01-15T14:30:01Z [INFO] [scraper-service] [processQuery] [abc123-def456] Processing query: "sk-" "openai" extension:env
2024-01-15T14:30:05Z [INFO] [scraper-service] [StoreDiscoveredKey] [abc123-def456] New key discovered: sk-****...****
```

**Log Levels**:
- **INFO**: Normal operations (MongoDB queries, RabbitMQ events, GitHub API calls)
- **ERROR**: Failures and errors (connection failures, validation errors, retry exhaustion)

### RabbitMQ Messages

**keys.discovered** topic:
```json
{
  "id": "507f1f77bcf86cd799439011",
  "key_value": "sk-proj-abc123...",
  "provider": "OpenAI",
  "repo_url": "https://github.com/user/repo",
  "file_path": "config/.env",
  "discovered_at": "2024-01-15T10:30:00Z"
}
```

**keys.validated** topic:
```json
{
  "id": "507f1f77bcf86cd799439011",
  "provider": "OpenAI",
  "status": "Valid",
  "validated_at": "2024-01-15T11:00:00Z"
}
```

## Troubleshooting

### Service Won't Start

**Symptom**: Service exits immediately after starting

**Possible Causes**:
1. Missing or invalid environment variables
2. MongoDB connection failure
3. RabbitMQ connection failure

**Solution**:
```bash
# Check logs
docker-compose logs scraper-service

# Verify environment variables
docker-compose exec scraper-service env | grep -E "MONGO|RABBITMQ|GITHUB"

# Test MongoDB connection
docker-compose exec mongodb mongosh --eval "db.adminCommand('ping')"

# Test RabbitMQ connection
docker-compose exec rabbitmq rabbitmqctl status
```

### No Keys Being Discovered

**Symptom**: Scraping cycles complete but no keys found

**Possible Causes**:
1. No enabled search queries
2. GitHub API rate limit exhausted
3. Invalid GitHub tokens

**Solution**:
```bash
# Check enabled queries
curl -H "Authorization: Bearer <session_token>" http://localhost:8080/api/v1/config/queries

# Check GitHub rate limit
curl http://localhost:8888/metrics | jq '.github_rate_limit'

# Verify GitHub tokens
curl -H "Authorization: Bearer ghp_token1" https://api.github.com/user
```

### Keys Not Being Validated

**Symptom**: Keys remain in "Pending" status

**Possible Causes**:
1. Worker service not running
2. RabbitMQ connection failure
3. Provider API timeouts

**Solution**:
```bash
# Check worker service status
docker-compose ps worker-service

# Check worker service logs
docker-compose logs worker-service | grep -i verifier

# Check RabbitMQ queues
docker-compose exec rabbitmq rabbitmqctl list_queues
```

### High Error Rate

**Symptom**: Many errors in logs or metrics

**Possible Causes**:
1. GitHub API rate limit exceeded
2. Provider API timeouts
3. Network connectivity issues

**Solution**:
```bash
# Check error details in logs
docker-compose logs scraper-service | grep ERROR

# Check GitHub rate limit status
curl http://localhost:8888/metrics | jq '.github_rate_limit'

# Increase scraping interval to reduce rate limit pressure
# Edit .env: SCRAPING_INTERVAL_MINUTES=30
docker-compose restart scraper-service
```

### MongoDB Connection Issues

**Symptom**: Health check returns 503, logs show MongoDB errors

**Solution**:
```bash
# Check MongoDB status
docker-compose ps mongodb

# Restart MongoDB
docker-compose restart mongodb

# Check MongoDB logs
docker-compose logs mongodb

# Verify MongoDB URI in .env
echo $MONGO_URI
```

### RabbitMQ Connection Issues

**Symptom**: Health check returns 503, logs show RabbitMQ errors

**Solution**:
```bash
# Check RabbitMQ status
docker-compose ps rabbitmq

# Restart RabbitMQ
docker-compose restart rabbitmq

# Check RabbitMQ logs
docker-compose logs rabbitmq

# Verify RabbitMQ URL in .env
echo $RABBITMQ_URL
```

### Graceful Degradation

The service is designed to continue operating even when RabbitMQ is unavailable:
- Scraping continues on schedule
- Validation continues on schedule
- Errors are logged but don't stop processing
- Services rely on scheduled tasks as fallback

This ensures the system remains operational during transient failures.

## Security Considerations

### API Key Protection

- Keys are masked in API responses (only first/last 4 characters shown)
- Keys are never logged in plaintext
- MongoDB access should be restricted to internal network
- Consider enabling MongoDB encryption at rest for production

### GitHub Token Security

- Tokens loaded from environment variables only
- No token storage in database
- Use separate tokens for different environments
- Rotate tokens regularly

### Authentication

- All API endpoints require session-based authentication
- Health and metrics endpoints are public (read-only)
- Session middleware validates requests before processing

## Performance Tuning

### Scraping Performance

- **Concurrent Queries**: All enabled queries run concurrently using goroutines
- **Rate Limiting**: 5-second delay between GitHub API requests (configurable)
- **Token Rotation**: Multiple GitHub tokens can be provided for higher throughput
- **Retry Logic**: Exponential backoff for failed requests (1s, 2s, 4s)

### Validation Performance

- **Concurrent Validation**: Up to 5 keys validated concurrently
- **Timeout**: 30-second timeout per provider request
- **Retry Logic**: Fixed 2-second delay for failed validations (up to 3 retries)

### Database Performance

- **Indexes**: Unique index on `key_value`, indexes on `status`, `provider`, `created_at`
- **Pagination**: API endpoints support pagination (10 keys per page)
- **Connection Pooling**: Reuses existing MongoDB connection pool

## License

This service is part of the project-phoenix-v2 microservices architecture.
