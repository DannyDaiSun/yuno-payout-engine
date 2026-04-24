# Bangkok Settlement Maze — Multi-Acquirer Payout Engine

A Go HTTP service that ingests settlement reports from three Thai acquirers (each with a different file format and schedule), normalizes them into a unified record shape, reconciles them against the merchant's transaction ledger, and answers cash-flow / fee / overdue queries via JSON endpoints. Built as the deliverable for the Yuno backend coding challenge.

## What It Does

LotusMarket processes ~50K transactions per day across three acquirers (ThaiAcquirer, GlobalPay, PromptPayProcessor). Each acquirer sends settlement reports in a different format on a different schedule. This service:

1. **Ingests** CSV and JSON settlement files via HTTP upload (one endpoint per acquirer).
2. **Normalizes** each row into a single `SettlementRecord` shape (`int64` satang amounts, Bangkok-local dates, canonical acquirer enum).
3. **Reconciles** settlements against the transaction ledger: marks each transaction `settled`, `pending`, or `overdue` and flags discrepancies (unknown txn, acquirer mismatch, amount mismatch).
4. **Answers** finance-team queries: tomorrow's expected cash inflow, unsettled transactions in the last N days, fees per acquirer for a month, currently overdue transactions, and (stretch) fee anomalies.

## Architecture

```
                 +-----------------------------+
                 |   POST /ingest/transactions |
                 |   POST /ingest/settlements  |
                 +--------------+--------------+
                                |
                                v
   +----------------+    +----------------+    +-----------------+
   | ThaiAcquirer   |    | GlobalPay      |    | PromptPay JSON  |
   | CSV parser     |    | CSV parser     |    | parser          |
   | (YYYY-MM-DD)   |    | (DD/MM/YYYY)   |    | (RFC3339)       |
   +-------+--------+    +-------+--------+    +--------+--------+
           |                     |                      |
           +----------+----------+----------+-----------+
                      |                     |
                      v                     v
              +---------------+    +-----------------+
              | normalized    |    | normalized      |
              | Transaction   |    | SettlementRecord|
              | (int64 satang)|    | (int64 satang)  |
              +-------+-------+    +--------+--------+
                      |                     |
                      v                     v
                  +-------------------------------+
                  |   in-memory Store (RWMutex)   |
                  +---------------+---------------+
                                  |
                                  v
                        +-------------------+
                        | Reconcile         |
                        | (txn x settlement |
                        |  -> status +      |
                        |  discrepancies)   |
                        +---------+---------+
                                  |
                                  v
                        +-------------------+
                        | Query Service     |
                        | cashflow / fees / |
                        | unsettled / over  |
                        +---------+---------+
                                  |
                                  v
                        +-------------------+
                        | Gin HTTP / JSON   |
                        +-------------------+
```

## Tech Stack

- **Go 1.25** (module `github.com/dannydaisun/payout-engine`)
- **Gin** for HTTP routing
- **In-memory store** guarded by `sync.RWMutex` (interface-friendly so a SQL-backed store can be swapped in)
- **`int64` satang** (THB minor units) end-to-end — never `float64` — so percentage fees and tier boundaries are exact
- **Bangkok timezone (Asia/Bangkok, UTC+7)** for every date boundary, business-day calculation, and month bucket

## Quick Start

### Run the demo script (proves all acceptance criteria)

```bash
cd project
./demo.sh
```

The demo script boots the server with the committed fixtures, runs every documented query, and prints pass/fail per challenge criterion.

### Manual setup

```bash
cd project
go mod download
go test -race -count=1 ./...           # ~60+ tests, all race-clean
go run ./cmd/gen-testdata              # regenerate fixtures (deterministic, seed=42)
go run ./cmd/server -seed ./data       # boots on :8080 with fixtures pre-loaded
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness check |
| POST | `/ingest/transactions` | Upload transactions CSV (`id,acquirer,amount,currency,transaction_date,payment_method`) |
| POST | `/ingest/settlements/:acquirer` | Upload a settlement file. `:acquirer` must be `ThaiAcquirer`, `GlobalPay`, or `PromptPayProcessor`. Body format is dispatched by acquirer. |
| GET | `/queries/cashflow?date=YYYY-MM-DD` | Expected net cash inflow on the given date, grouped by acquirer. Default: tomorrow Bangkok. |
| GET | `/queries/unsettled?days=7&as_of=YYYY-MM-DD` | Unsettled (pending or overdue) transactions from the past N days. Default `days=7`, `as_of=today Bangkok`. |
| GET | `/queries/fees?month=YYYY-MM` | Total fees per acquirer for the month (Bangkok boundaries). |
| GET | `/queries/overdue?as_of=YYYY-MM-DD` | Transactions whose expected settlement date has passed without a matching settlement. |
| GET | `/queries/anomalies` | Stretch goal: settlements whose fee deviates from the acquirer's expected fee rule. |

### Example Queries

```bash
# Tomorrow's expected cash, by acquirer
curl -s "http://localhost:8080/queries/cashflow?date=2026-04-25"
# {
#   "date": "2026-04-25",
#   "currency": "THB",
#   "by_acquirer": [
#     {"acquirer":"ThaiAcquirer","net_amount":"124000.50"},
#     {"acquirer":"GlobalPay","net_amount":"87520.00"},
#     {"acquirer":"PromptPayProcessor","net_amount":"43010.25"}
#   ],
#   "total": "254530.75"
# }

# Fees per acquirer for April 2026
curl -s "http://localhost:8080/queries/fees?month=2026-04"

# Anything overdue as of a specific date
curl -s "http://localhost:8080/queries/overdue?as_of=2026-04-24"

# Last 7 days of unsettled transactions
curl -s "http://localhost:8080/queries/unsettled?days=7&as_of=2026-04-24"

# Upload a Thai settlement CSV
curl -s -X POST --data-binary @data/settlements/thai_acquirer.csv \
  http://localhost:8080/ingest/settlements/ThaiAcquirer
```

## Acquirer Rules

| Acquirer | Schedule | File format | Date format | Fee structure |
|----------|----------|-------------|-------------|---------------|
| **ThaiAcquirer** | Daily, next business day | CSV (`txn_ref,transaction_date,settlement_date,gross_amt,fee_amt,net_amt,payment_method`) | `YYYY-MM-DD` | 2.5% of gross |
| **GlobalPay** | Tuesday + Friday only (same-day ineligible — see note) | CSV (`reference_number,processed_on,payout_date,original_amount,processing_fee,settled_amount,type`) | `DD/MM/YYYY` | `10 THB + 2%` |
| **PromptPayProcessor** | T+3 business days | JSON array (`transaction_id,txn_date,settle_date,amount,merchant_fee,net_payout,channel`) | RFC3339 (any TZ; normalized to Bangkok) | Tiered: `<5K` 1.5%, `5K-20K` 1.8%, `>20K` 2.2% |

**GlobalPay same-day policy:** a transaction processed on Tuesday is *not* eligible for that same Tuesday's batch (batches close at 00:00 Bangkok); it settles the following Friday. Documented and tested.

**Public holidays:** only Saturday/Sunday are treated as non-business days. Bank of Thailand holiday calendar is future work.

## Reconciliation Statuses

- **settled** — a settlement record exists for `(txn_id, acquirer)`
- **pending** — no settlement, expected settlement date is in the future relative to `as_of`
- **overdue** — no settlement, expected settlement date is on or before `as_of`

## Discrepancies Detected

- `unknown_transaction` — settlement references a transaction ID we have not seen
- `acquirer_mismatch` — settlement acquirer differs from the transaction's acquirer for the same ID
- `amount_mismatch` — settlement gross differs from the transaction amount

## Trade-offs (Honest)

- **In-memory store.** Chosen for prototype speed. Production would back the same `Store` interface with PostgreSQL. Nothing persists across restarts; fixtures are reloaded from disk.
- **Public holidays not handled.** Only weekends are treated as non-business days. Production would integrate the Bank of Thailand holiday calendar.
- **HTTP file upload (no SFTP / dir watcher).** The spec accepts upload as one of three options; we picked it because it is the easiest to demo and to test end-to-end.
- **No authentication.** Explicitly out of scope per the spec.
- **Single currency (THB).** Codified in the parser; non-THB rows are rejected. Multi-currency would extend the money helpers.
- **Refunds / negative amounts not modeled.** Parsers reject negatives; a refund engine is out of scope.
- **Partial / multiple settlements per transaction not modeled.** A duplicate `(txn_id, acquirer)` is treated as idempotent at the store layer; reconciliation flags it as a discrepancy.

## Project Layout

```
project/
├── cmd/
│   ├── server/             # HTTP server entry point + end-to-end integration test
│   └── gen-testdata/       # deterministic fixture generator (seed=42)
├── internal/
│   ├── domain/             # types, money helpers, Bangkok TZ helpers, constants
│   ├── schedule/           # ExpectedSettlementDate per acquirer (business-day math)
│   ├── ingest/             # 3 parsers: Thai CSV, GlobalPay CSV, PromptPay JSON
│   ├── store/              # thread-safe in-memory store (sync.RWMutex)
│   ├── reconcile/          # match settlements <-> txns, flag discrepancies
│   ├── query/              # cashflow / unsettled / fees / overdue / anomalies
│   ├── api/                # gin HTTP handlers + transactions CSV loader
│   └── anomaly/            # stretch: fee anomaly detection
└── data/                   # 300 txns + 3 settlement files (committed fixtures)
    ├── transactions.csv
    └── settlements/
        ├── thai_acquirer.csv
        ├── global_pay.csv
        └── promptpay.json
```

## Test Coverage

~60 tests across 12 packages, all race-clean. See `TEST_CASES.md` for the full inventory.

```bash
go test -race -count=1 ./...
```

## Further Reading

- `SYSTEM_OVERVIEW.md` — module dependency graph, design rationale, future work
- `TEST_CASES.md` — full test inventory grouped by sanity / regression / integration / negative
- `AI_PROMPT_LOG.md` — AI tool usage and workflow notes
- `behaviors.md` — original test-list (planned + implemented) used to drive the build
