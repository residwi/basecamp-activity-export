# Claude Code Guidance

This file provides context for Claude Code (Anthropic's CLI) when working on this project. It's not meant for general documentation—see README.md for that.

## Project Overview

`bclog` is a CLI tool that exports Basecamp 3 activity data into organized markdown reports. It aggregates your contributions scattered across projects into a single, searchable document - useful for performance reviews, project retrospectives, or personal records.

The tool tracks multiple activity types:

- **Messages created** - posts and show-and-tells
- **Todos** - created, completed, and assigned
- **Todo lists created** - collections of todos
- **Kanban cards** - created and assigned
- **Comments** - written on any item
- **Question answers** - daily check-in responses

Key features:

- **Smart organization**: Groups activities by epic and project with direct Basecamp links
- **Rich formatting**: Converts HTML descriptions to clean Markdown
- **Performance optimized**: Concurrent fetching (6 goroutines), smart pagination (gallop + binary search cuts API calls ~70%), HTTP caching with ETag support
- **Respectful API usage**: Token bucket rate limiting (50 req/10s), exponential backoff retry
- **Cache management**: Commands to view stats and clear cache when needed
- **OAuth authentication**: Secure browser-based login with automatic token refresh

## Development Commands

### Building

```bash
# Build the binary
make build

# Or build directly
go build -o bin/bclog ./cmd/bclog
```

### Testing

```bash
# Run all tests with race detector
make test

# Run tests with coverage report (generates coverage.html)
make test-coverage

# CI test with 90% coverage threshold enforcement
make test-ci

# Run a specific test (auto-loads .env)
bin/run_test TestName

# Example: test OAuth authorization flow
bin/run_test TestOAuthClient_Authorize
```

### Dependencies and Code Quality

```bash
# Download and tidy dependencies
make deps

# Format check
gofmt -l .

# Vet check
go vet ./...
```

## Architecture Overview

### Architectural Approach

This project uses **feature-based architecture** (also known as vertical slice architecture), where code is organized by business capability rather than technical layer.

**What this means:**

- Each feature directory (`auth/`, `cache/`, `export/`) contains all code related to that feature
- Features are self-contained with their own implementation, tests, and utilities
- Related functionality stays together instead of being split across layers

**Contrast with layered architecture:**

- ❌ Layered: `controllers/auth.go`, `services/auth.go`, `models/auth.go`
- ✅ Feature-based: `auth/oauth_client.go`, `auth/oauth_callback.go`, `auth/oauth_token.go`

**Benefits for this project:**

- **Easy navigation**: All OAuth code is in `auth/`, not scattered across layers
- **Clear boundaries**: Each command (`auth`, `cache`, `export`) is independently testable
- **Reduced coupling**: Features don't share internal implementation details
- **Simpler onboarding**: New developers can understand one feature at a time

### Package Structure

The codebase is organized into feature-based modules under `internal/command/` and shared infrastructure packages:

**Feature Modules:**

- **internal/command/auth/**: Complete OAuth implementation (client, callback server, token management)
- **internal/command/cache/**: Cache management operations and utilities
- **internal/command/export/**: Export orchestration with report generation and concurrent resource fetching

**Infrastructure & Shared Packages:**

- **cmd/bclog/**: Main binary entry point
- **internal/basecamp/**: Core API client with middleware chain, smart pagination, concurrent fetching
- **internal/activity/**: Domain model (UserActivity aggregate, filtering logic)
- **internal/trace/**: Observability (span tracking, network stats, thread-safe for concurrent goroutines)
- **internal/httpcache/**: HTTP caching layer (ETag/Last-Modified support, file-based storage)
- **internal/progress/**: Real-time progress display during export
- **internal/config/**: Configuration management

### Key Architectural Patterns

**Middleware Chain Pattern**
The HTTP client uses a layered middleware approach where each layer wraps the next:

- **Tracking**: Records request metrics and timing
- **Cache**: Handles ETag/Last-Modified caching
- **Retry**: Exponential backoff with jitter for 429/5xx errors
- **RateLimit**: Token bucket to stay under API limits (50 requests per 10 seconds)

This keeps concerns separated and makes each layer independently testable.

**Concurrent Fetching Strategy**
6 goroutines fetch different resource types (messages, todos, todolists, cards, comments, question answers) concurrently using `errgroup` for coordination. This parallelizes the export process while maintaining error handling.

**Smart Pagination**
Instead of fetching all pages blindly, the system uses:

1. **Gallop search**: Tests pages 1, 2, 4, 8, 16... to quickly find boundaries
2. **Binary search**: Narrows down to the exact page range containing the target year
3. **Concurrent page fetching**: Only fetches relevant pages in parallel

This approach cuts API calls by ~70% compared to naive pagination.

**Observability via Trace Package**
The `internal/trace` package tracks:

- Spans (pagination, fetching, filtering operations)
- Network stats (request counts, cache hits, rate limit waits)
- Pagination metrics (gallop steps, binary search iterations)

All tracking is thread-safe for concurrent goroutine usage.

## Important Implementation Details

### Connection Pooling

- `MaxIdleConnsPerHost=20` (default is 2) to avoid connection churn during concurrent bursts
- Keeps connections alive for reuse across the 6 concurrent fetching goroutines

### OAuth Token Storage

- Location: `~/.config/bclog/token.json`
- Contains access token, refresh token, and expiry
- Managed by `internal/command/auth/oauth_token.go`

### HTTP Cache Storage

- Location: `~/.bclog/cache/`
- File-based cache managed by `internal/httpcache/file_manager.go`
- Supports ETag and Last-Modified headers per Basecamp API spec

### Rate Limiting

- 50 requests per 10 seconds (Basecamp's limit)
- Token bucket implementation in middleware chain
- Automatically waits when limit is approached

### Basecamp Geared Pagination

- Basecamp supports different page sizes: 15, 30, 50, 100 items per page
- The client uses page size 100 by default to minimize request count
- Pagination metadata comes from `X-Total-Count` header

### Thread Safety

- `internal/trace` package uses mutexes for concurrent access
- `internal/activity/activity.go` UserActivity uses mutex for safe concurrent updates
- All goroutines coordinate via `errgroup` to propagate errors correctly
