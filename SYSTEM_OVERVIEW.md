# System Overview

Architecture deep-dive for the Bangkok Settlement Maze payout engine. Companion to `README.md`.

## Module Dependency Graph

```
                    +----------------+
                    |    domain      |   (types, money, BangkokTZ)
                    +-------+--------+
                            ^
        +-------------------+-------------------+-----------+----------+
        |                   |                   |           |          |
   +----+----+        +-----+-----+        +----+----+  +--+--+   +---+---+
   | schedule|        |  ingest   |        |  store  |  |hello|   |anomaly|
   +----+----+        +-----+-----+        +----+----+  +-----+   +-------+
        ^                   ^                   ^                       ^
        |                   |                   |                       |
        |                   |              +----+-----+                 |
        |                   |              |reconcile |                 |
        |                   |              +----+-----+                 |
        |                   |                   ^                       |
        |                   |                   |                       |
        +-------------------+----+--------------+-----------------------+
                                 |
                            +----+----+
                            |  query  |
                            +----+----+
                                 ^
                                 |
                            +----+----+
                            |   api   |   (gin handlers)
                            +----+----+
                                 ^
                                 |
                       +---------+---------+
                       |   cmd/server      |  (main + integration test)
                       +-------------------+

   +------------------+
   | cmd/gen-testdata |   (standalone — writes raw fixture files,
   +------------------+    no domain dep, deterministic seed=42)
```

**Key invariants:**

- `domain` has zero internal dependencies (pure types + helpers).
- `cmd/gen-testdata` deliberately does *not* import `internal/domain` — it writes raw bytes to disk so fixture format drift is caught by parser tests rather than masked by shared code.
- `api` is the only package that touches `gin`. Swapping in `chi` or `net/http` would touch only `internal/api` and `cmd/server`.
- `store` is only used by `api`, `query`, `reconcile`, and `cmd/server`. Replacing it with a `*sql.DB` repository implementing the same method set is a one-package change.

## Design Decisions

### Why in-memory store

A 2-hour prototype scoring rubric rewards a complete pipeline with tests over deep infrastructure. The challenge spec says "stored in a queryable format" — not "durably persisted". An in-memory `sync.RWMutex`-guarded map gives us:

- Zero setup friction in tests and demos
- Deterministic state (boot from fixtures every time)
- A clean `Store` surface that maps 1:1 onto a future SQL implementation

The trade-off — no persistence across restarts — is documented and accepted.

### Why `int64` satang (minor units)

Percentage fees and tier boundaries break under `float64`. Examples that bit us in design:

- 1.5% of `4999.99 THB` vs 1.8% of `5000.00 THB` — tier-boundary precision matters for the PromptPay anomaly detector.
- `gross - fee != net` rounding deltas in source files must be detectable to ±1 minor unit.

We parse `"1000.25"` to `int64(100025)` and never leave that representation until we format JSON output. All percentage math uses basis points (`grossMinor * bps / 10000`).

### Why Bangkok TZ throughout

The spec states Bangkok times. Subtle bugs that would result from naive `time.Now()`:

- A transaction at 23:59 Bangkok on April 30 is actually April 30 *17:59 UTC*. A naive UTC bucket places it in April; a naive UTC+7 reading places it on May 1. We deliberately bucket by Bangkok-local midnight (`BangkokMidnight(t)`) so the merchant's monthly fee report matches their accounting close.
- "Tomorrow" for cashflow is Bangkok-tomorrow, not server-tomorrow. The default `/queries/cashflow` uses `BangkokMidnight(now()).AddDate(0,0,1)`.
- Business-day skips operate on Bangkok weekday, not UTC weekday — relevant for the GlobalPay Tuesday/Friday rule near midnight.

`domain.BangkokTZ()` and `domain.BangkokMidnight()` are the single source of truth.

## Concurrency Model

- The store uses `sync.RWMutex`: many readers, one writer. Reconciliation and query reads do not block each other.
- `TestStoreConcurrentSaveAndRead` exercises 100 goroutines hammering writes + reads under `-race`.
- Gin handlers are stateless; per-request data flows through method parameters.
- Reconciliation is a pure function `Reconcile(txns, settlements, asOf) -> ReconcileResult`. It does not mutate the store; the store only holds raw inputs.

## Error Handling Patterns

Three layers, each with a distinct responsibility:

1. **Parser layer** returns `error` with file + row context. Examples:
   - "ThaiAcquirer CSV missing required column fee_amt"
   - "PromptPay JSON: transaction_id is null at index 2"
   - Empty file, malformed JSON, non-numeric amount, negative amount, > 2 decimals — all rejected with a clear message.
2. **Reconciliation layer** never errors on a per-record mismatch. It produces `Discrepancy` records (`unknown_transaction`, `acquirer_mismatch`, `amount_mismatch`) so finance can investigate without losing other valid rows.
3. **HTTP layer** maps errors into structured JSON: `{"error": "<code>", "message": "<human>"}`. Each handler picks an appropriate 4xx code:
   - `invalid_settlement_file` (400) — parser rejected the body
   - `unknown_acquirer` (400) — `:acquirer` path param not recognized
   - `invalid_date` / `invalid_month` / `invalid_days` (400) — query param parsing
   - 5xx is reserved for genuinely unexpected errors

## Discrepancy Detection Design

The reconciliation pass is the only place that joins raw inputs. It produces:

- `[]ReconciledTransaction` — every known transaction tagged settled / pending / overdue
- `[]Discrepancy` — settlement records that don't match cleanly

Discrepancy reasons are kept narrow on purpose:

- `unknown_transaction` — settlement references a transaction ID not in the ledger. Could mean the txn upload is missing, the settlement file has stale data, or a typo.
- `acquirer_mismatch` — a settlement's acquirer differs from the transaction's acquirer for the same ID. This is a hard data-integrity flag.
- `amount_mismatch` — settlement gross differs from transaction amount. Could indicate refund handling, partial settlement (out of scope), or a data error.

Fee deltas inside a single record (`gross - fee != net`) are recorded on the settlement at parse time, not bubbled through reconciliation, because the source-of-truth fee is whatever the acquirer says it is. The stretch *anomaly* detector compares actual fees to the expected fee per the acquirer's published rule — that's a separate concern surfaced via `/queries/anomalies`.

## Fixture Generation

`cmd/gen-testdata` writes deterministic fixtures with `rand.New(rand.NewSource(42))`:

- 300 transactions, 100 per acquirer, over the past 30 days
- Amounts uniform `100-50000 THB`
- 4 payment methods: credit_card, promptpay, truemoney_wallet, bank_transfer
- Settlement files cover ~70 transactions per acquirer (mix of settled / pending / overdue)
- Settlement dates use real business-day math so reconciliation reflects realistic timing

Output is committed to `data/` so the repo can be cloned and demoed without running the generator. `TestGenerationIsDeterministicWithSeed` guards reproducibility.

## Future Work

| Area | Today | Production direction |
|------|-------|----------------------|
| Persistence | In-memory `map` behind `sync.RWMutex` | PostgreSQL behind same `Store` interface; add migrations |
| Holiday calendar | Sat/Sun only | Integrate Bank of Thailand calendar; cache annually |
| Multi-currency | THB only | Currency-aware money helpers; FX-rate ingestion |
| File transport | HTTP upload | SFTP poller + S3 inbound + webhooks per acquirer |
| Auth | None | API key per merchant + per-acquirer scoped tokens |
| Observability | `log.Printf` | structured logs (zerolog/zap) + OTel traces + Prom metrics |
| Refund / chargeback | Out of scope | Negative-amount records + linked-record schema |
| Partial settlements | Idempotent dedupe at store; flagged as discrepancy | First-class `SettlementBatch` aggregate |
| Anomaly detection | Single-pass fee deviation | Time-series baseline per acquirer + per payment method; alerting webhook |
| Cash-flow forecast | Not implemented | 7-14 day forecast using `ExpectedSettleDate` + historical settlement-rate priors |
| Concurrency on ingest | Single-process | Per-acquirer queues; idempotent retries; outbox pattern |
