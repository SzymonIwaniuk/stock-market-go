# Stock Market Service

A simplified stock market REST API built in Go. Wallets can buy and sell stocks from a central Bank, with all successful operations recorded in an audit log.

## Architecture

The service runs as 3 identical Go instances behind an Nginx reverse proxy, with shared state in Redis. This provides high availability — killing any single instance (via `POST /chaos`) doesn't take down the service. Docker Compose `restart: always` automatically recovers killed instances.

```
                     ┌──────────┐
  localhost:PORT ──▶ │  Nginx   │
                     └────┬─────┘
                ┌─────────┼─────────┐
                ▼         ▼         ▼
            ┌──────┐  ┌──────┐  ┌──────┐
            │ App1 │  │ App2 │  │ App3 │
            └──┬───┘  └──┬───┘  └──┬───┘
               └──────────┼────────┘
                          ▼
                     ┌─────────┐
                     │  Redis  │
                     └─────────┘
```

Trade operations (buy/sell) are atomic via Redis Lua scripts, preventing race conditions across instances.

## Prerequisites

- Go runtime
- Docker and Docker Compose

## Quick Start

```bash
go run start.go <PORT>
```

For example:

```bash
go run start.go 8080
```

The service will be available at `http://localhost:8080`. Works on Windows, Linux, and macOS (both x64 and arm64).

To stop:

```bash
docker compose down
```

## API Endpoints

### POST /stocks

Set the bank's stock inventory.

```bash
curl -X POST http://localhost:8080/stocks \
  -d '{"stocks":[{"name":"AAPL","quantity":10},{"name":"GOOG","quantity":5}]}'
```

### GET /stocks

Get the current bank state.

```bash
curl http://localhost:8080/stocks
```

Response:

```json
{"stocks":[{"name":"AAPL","quantity":10},{"name":"GOOG","quantity":5}]}
```

### POST /wallets/{wallet_id}/stocks/{stock_name}

Buy or sell a single stock. Creates the wallet automatically if it doesn't exist.

```bash
# Buy
curl -X POST http://localhost:8080/wallets/w1/stocks/AAPL -d '{"type":"buy"}'

# Sell
curl -X POST http://localhost:8080/wallets/w1/stocks/AAPL -d '{"type":"sell"}'
```

| Status | Condition |
|--------|-----------|
| 200 | Success |
| 400 | No stock in bank (buy) or wallet (sell), or invalid type |
| 404 | Stock doesn't exist in bank registry |

### GET /wallets/{wallet_id}

Get a wallet's current holdings.

```bash
curl http://localhost:8080/wallets/w1
```

Response:

```json
{"id":"w1","stocks":[{"name":"AAPL","quantity":3}]}
```

### GET /wallets/{wallet_id}/stocks/{stock_name}

Get the quantity of a specific stock in a wallet.

```bash
curl http://localhost:8080/wallets/w1/stocks/AAPL
```

Response: `3`

### GET /log

Get the full audit log of all successful wallet operations, in order.

```bash
curl http://localhost:8080/log
```

Response:

```json
{"log":[{"type":"buy","wallet_id":"w1","stock_name":"AAPL"},{"type":"sell","wallet_id":"w1","stock_name":"AAPL"}]}
```

### POST /chaos

Kill the instance that handles this request. The service remains available through the other instances.

```bash
curl -X POST http://localhost:8080/chaos
```

## Running Tests

### Unit tests

Uses [miniredis](https://github.com/alicebob/miniredis) -- a pure Go in-memory Redis server. No external dependencies needed.

```bash
go test ./... -v
```

### End-to-end tests

Requires the service to be running (via `go run start.go <PORT>`). Tests hit the live API using [testify/suite](https://github.com/stretchr/testify).

```bash
PORT=8080 go test ./e2e_tests/ -tags=e2e -v
```

## Project Structure

```
├── cmd/server/main.go          Entry point, routing, graceful shutdown
├── internal/
│   ├── handler/                HTTP handlers (wallet, stock, log, chaos, health, reset)
│   ├── model/                  Domain types and DTOs
│   ├── store/                  Store interface + Redis implementation
│   └── middleware/             Request logging
├── e2e_tests/                  End-to-end API tests (build tag: e2e)
├── nginx/nginx.conf            Nginx reverse proxy config
├── docker-compose.yml          3 app instances + nginx + redis
├── Dockerfile                  Multi-stage Go build
├── start.go                    Cross-platform startup command
└── README.md
```

## Design Decisions

- **Redis for shared state**: Lightweight, runs in Docker, provides atomic operations via Lua scripts. No external assumptions beyond Docker.
- **Lua scripts for trades**: Buy and sell operations atomically check preconditions, update bank, update wallet, and append to the audit log in a single Redis operation. This prevents race conditions between concurrent instances.
- **chi router**: Lightweight, idiomatic Go router with path parameters and middleware support. No heavy frameworks.
- **No mocks in tests**: Tests run against a real Redis-compatible server (miniredis), exercising the actual store implementation including Lua scripts.
- **Nginx with `proxy_next_upstream`**: If one app instance is down, nginx automatically retries the request on another instance.
