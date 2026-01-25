# Basecamp Activity Export

`bclog` is a CLI tool that exports your Basecamp 3 activity into a clean, organized markdown log.

It grabs things like:

- Messages and comments
- Todo items and todo lists
- Cards and question answers

## The Why

I built this because yearly performance reviews are stressful enough without having to play archaeologist with my own data.

Basecamp is great for the day-to-day. Todos, messages, show and tells, stories, epics. But when I needed to answer "What did I actually ship this year?", I hit a wall. My work was scattered across dozens of projects, buried in "Show and Tells," hidden in closed Todo lists, or lost in long comment threads.

I didn't want to spend hours clicking through project after project and scrolling endlessly to find that one epic I led back in February. so I built `bclog`. It automates the digging and gives me a single file containing every story, epic, and contribution I made, so I have the receipts when review season comes around.

## What It Does

- Pulls all your activity from Basecamp 3 API
- Grabs messages, todos, cards, comments — all in one go
- Groups everything by epic and project
- Includes descriptions and links so you can actually verify things
- Caches requests so you're not hammering the server unnecessarily

It's also reasonably fast:

- Fetches different resource types concurrently
- Smart [pagination](https://github.com/basecamp/bc3-api?tab=readme-ov-file#pagination) with gallop search + binary search cuts API calls by ~70%
- Respects [rate limits](https://github.com/basecamp/bc3-api?tab=readme-ov-file#rate-limiting-429-too-many-requests) (50 requests per 10 seconds)
- Uses [HTTP caching](https://github.com/basecamp/bc3-api?tab=readme-ov-file#using-http-caching) with ETag support

## Getting Started

### Prerequisites

- Go 1.25.0 or higher
- Basecamp 3 account with activity data
- Basecamp OAuth credentials (register at <https://launchpad.37signals.com/integrations>)

### Installation

```bash
# Clone repository
git clone https://github.com/residwi/basecamp-activity-export.git
cd basecamp-activity-export

# Install dependencies
make deps

# Build binary
make build
```

Or build directly:

```bash
go build -o bin/bclog ./cmd/bclog
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

Tokens and cache live here:

- OAuth tokens: `~/.config/bclog/token.json`
- HTTP cache: `~/.bclog/cache/`

### Usage

```bash
# Log in via browser
./bin/bclog login

# Export this year's activity
./bin/bclog export

# Export a specific year
./bin/bclog export --year 2025

# Check cache stats
./bin/bclog cache stats

# Clear HTTP cache
./bin/bclog cache clear

# Logout and clear cached token
./bin/bclog logout
```

## Development

```bash
# Build binary
make build

# Run tests
make test

# Run tests with coverage report
make test-coverage

# CI check with coverage threshold
make test-ci

# Run a specific test
bin/run_test TestOAuthClient_Authorize

# Dependencies
make deps

# Code quality checks
gofmt -l .
go vet ./...
```

### Under the Hood

If you're digging into the code, here are a few things to look out for:

#### **Smart Pagination**

The API provides an `X-Total-Count` header, but we usually don't need the whole history, just the page with specific target year only. Instead of fetching everything blindly, we use gallop search (testing pages 1, 2, 4, 8, 16...) combined with binary search to find the last relevant page. Then we fetch only those pages concurrently. This cuts API calls by ~70% in typical cases.

#### **Concurrency**

6 goroutines fetch different resource types (messages, todos, todolists, cards, comments, question answers) concurrently using errgroup for coordination and error handling.

#### **Connection Pooling**

To avoid connection churn during the burst of concurrent requests, we force MaxIdleConnsPerHost=20 (default is 2), keeping connections alive for reuse.

#### **Observability**

The internal trace package tracks spans (pagination, fetching, filtering), network stats (request counts, cache hits, rate limit waits), and pagination metrics — all thread-safe for concurrent use.

#### **Middleware Chain**

The HTTP client stacks middleware where each layer does one thing:

- **Tracking**: records request metrics and timing
- **Cache**: handles ETag/Last-Modified caching
- **Retry**: exponential backoff with jitter for 429/5xx errors
- **RateLimit**: token bucket to stay under API limits

Each one wraps the next, keeping things separated and testable.

## License

[MIT License](LICENSE)
