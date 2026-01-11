# Export Basecamp Activity For Performance Review

`bcreview` is a CLI tool that generates comprehensive performance review reports from Basecamp 3 activity data.

It fetches user contributions across multiple resource types, such as:

- Messages and comments
- Todo items and todo lists
- Cards and question answers

The tool exports activity data as organized markdown reports with full descriptions, direct links to Basecamp resources, and epic categorization.

## Features

Manual performance reviews require time-consuming aggregation of scattered activities across projects, tools, and teams. `bcreview` solves this by:

- **Automating data collection** - Fetches all user activity automatically via Basecamp 3 API
- **Providing comprehensive coverage** - Captures messages, todos, cards, and comments in a single report
- **Organizing by epics** - Groups activities under their respective projects and topics
- **Ensuring accuracy** - Includes full context with descriptions and direct links to source materials
- **Optimizing performance** - Uses parallel fetching, smart caching, and rate limiting to minimize API load

Performance benefits:

- Concurrency for parallel resource fetching
- Smart pagination reduces API calls by approximately 70%
- HTTP caching with ETag/Last-Modified support
- Sliding window rate limiter (50 requests per 10 seconds)

## Quick Start

### Prerequisites

- Go 1.24.0 or higher
- Basecamp 3 account with activity data
- Basecamp OAuth credentials (register at <https://launchpad.37signals.com/integrations>)

### Installation

```bash
# Clone repository
git clone https://github.com/residwi/bcreview.git
cd bcreview

# Install dependencies
make deps

# Build binary
make build
```

Or build directly without Make:

```bash
go build -o bin/bcreview ./cmd/bcreview
```

### Configuration

Copy `.env.example` to `.env`

```bash
cp .env.example .env
```

Edit `.env` to set your Basecamp OAuth credentials:

```env
# Required: Your Basecamp account ID
# Find this in the URL: https://3.basecampapi.com/{ACCOUNT_ID}/
BASECAMP_ACCOUNT_ID=your_account_id

# Required: OAuth credentials from https://launchpad.37signals.com/integrations
BASECAMP_CLIENT_ID=your_client_id
BASECAMP_CLIENT_SECRET=your_client_secret
```

Configuration files:

- OAuth tokens: `~/.config/bcreview/token.json`
- HTTP cache: `~/.bcreview/cache/`

### Usage

```bash
# Authenticate with Basecamp OAuth
./bin/bcreview login

# Generate performance review for current year
./bin/bcreview export

# Generate for specific year
./bin/bcreview export --year 2024

# View HTTP cache statistics
./bin/bcreview cache stats

# Clear HTTP cache
./bin/bcreview cache clear

# Logout and clear cached token
./bin/bcreview logout
```

## Development

```bash
# Build binary
make build

# Run tests (current coverage: 63.8%)
make test

# Generate test coverage report
make test-coverage

# Enforce coverage threshold (70%)
make test-ci

# Test specific case
bin/run_test TestOAuthClient_Authorize

# Dependency management
make deps

# Code quality checks
gofmt -l .      # Must return zero files
go vet ./...    # Must return zero warnings
```

### Key Patterns

**Pagination Strategy**: Exponential probing with binary search

1. Test pages [1, 2, 4, 8, 16, 32, 64, 128, 256] in parallel
2. Cancel higher page probes when boundary found
3. Binary search to refine exact boundary
4. Fetch all valid pages concurrently

**Rate Limiting**: Sliding window algorithm with O(1) complexity prevents burst-at-boundary issues

**Parallel Aggregation**: 6 goroutines fetch different resource types concurrently with `sync.WaitGroup` coordination and thread-safe error handling

**Context Propagation**: Uses `context.Context` to thread tracking and resource type information through the call chain

**HTTP Optimization**: Connection pooling with `MaxIdleConnsPerHost=50` to prevent connection churn

## License

MIT License - See LICENSE file for details.
