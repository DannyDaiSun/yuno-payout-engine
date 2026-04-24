# Payout Engine

Multi-acquirer payout engine built in Go.

## Quick Start

```bash
go run ./cmd/server
```

## Tests

```bash
./run_unit_tests.sh
```

## Docker

```bash
docker compose up postgres -d
docker compose --profile server up --build
```
