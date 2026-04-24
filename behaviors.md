# Behaviors — Bangkok Settlement Maze (Refined)

Spec → test names. In-memory store. Money = `int64` minor units (satang). All times in **Bangkok timezone (UTC+7)**.

---

## Disambiguation (resolved before parallel work)

1. **3 acquirers:** ThaiAcquirer (daily CSV), GlobalPay (Tue+Fri CSV), PromptPayProcessor (T+3 JSON)
2. **GlobalPay same-day policy:** transaction on Tue 23:59 → settles **next Friday** (not same Tue — same-day ineligible). Rationale: settlement batches close at midnight. Document in README.
3. **Dates in spec (YYYY-MM-DD etc) are Bangkok-local dates.** Normalize to Bangkok midnight internally.
4. **Public holidays:** ignored (future work). Only Sat+Sun treated as non-business days.
5. **Refunds / negative amounts:** out of scope. Parser rejects negatives.
6. **Partial settlements (>1 settlement per txn):** out of scope. Parser rejects duplicate settlements; reconcile flags as discrepancy.
7. **Currency:** always THB. Parser rejects non-THB.

---

## TDD Approach (COMPRESSED — 2hr scope)

**Not per-cycle RED→GREEN→REFACTOR.** Instead **test-list per module**:

1. Write ALL unit tests for a module (5 min) — tests fail compilation because impl doesn't exist
2. Implement module to pass ALL tests (10-15 min)
3. Commit when module green
4. Refactor ONCE at module-end if obvious smell — otherwise defer to post-challenge

Module-level RED → module-level GREEN. Skip the micro-loop. Still test-first discipline; just batched.

**Verification gate before claiming module done:**
```
go test -race -count=1 ./internal/{module}/...
```
Must be green and all tests pass. No "should work" claims.

---

## Codex Review Checkpoints (inlined into workflow)

| When | Codex Task |
|------|-----------|
| Pre-Sprint 1 (now) | ✅ Edge case audit — done |
| End of Sprint 1 | Review Ingest module correctness across all 3 parsers |
| End of Sprint 2 | Review Store thread-safety + Reconcile discrepancy logic |
| End of Sprint 3 | Review Query layer + HTTP API contract |
| End of Sprint 4 | Adversarial review of full pipeline |
| Sprint 5 (integration test) | Codex generates the integration test scenario |

---

## Module 0: Domain (FROZEN — main agent, first 5 min)

Path: `internal/domain/`

**Types:** `Acquirer`, `PaymentMethod`, `SettlementStatus`, `Transaction`, `SettlementRecord`, `ReconciledTransaction`, `Discrepancy`, `DiscrepancyReason`.

**Helpers:** `ParseMinorUnits(s string) (int64, error)`, `FormatMinorUnits(int64) string`, `BangkokTZ()` (returns `*time.Location`), `BangkokMidnight(t time.Time) time.Time`.

### Tests (5)

- [ ] `TestParseAmountToMinorUnits` — `"1000.25"` → `100025`
- [ ] `TestParseAmountRejectsNegative` — `"-5.00"` → error (P0)
- [ ] `TestParseAmountRejectsMoreThanTwoDecimals` — `"100.001"` → error (P0)
- [ ] `TestFormatMinorUnits` — `100025` → `"1000.25"`
- [ ] `TestBangkokMidnightNormalizes` — any `time.Time` → same day at 00:00 in UTC+7 (P0)

**Constants:** 3 acquirer IDs, 4 payment methods, 3 status values, discrepancy reasons.

---

## Module A: Schedule Calculator (subagent-A, independent)

Path: `internal/schedule/`

Signature: `func ExpectedSettlementDate(acquirer Acquirer, txnDate time.Time) (time.Time, error)`

### Tests (10 — 8 happy + 2 edge)

Happy paths:
- [ ] `TestThaiAcquirerNextBusinessDay` — Mon txn → Tue
- [ ] `TestThaiAcquirerFridaySkipsToMonday` — Fri → Mon
- [ ] `TestGlobalPayMondayGoesToTuesday` — Mon txn → Tue
- [ ] `TestGlobalPayWednesdayGoesToFriday` — Wed → Fri
- [ ] `TestGlobalPayTuesdaySkipsToFriday` — Tue txn (same-day ineligible) → Fri
- [ ] `TestGlobalPayFridaySkipsToNextTuesday` — Fri txn → next Tue
- [ ] `TestPromptPayT3Weekday` — Mon → Thu
- [ ] `TestPromptPayT3AcrossWeekend` — Wed → next Mon (skip Sat/Sun)

Edge cases (P0):
- [ ] `TestPromptPayFridayTxnSettlesWednesday` — Fri txn, +3 business days → next Wed
- [ ] `TestUnknownAcquirerReturnsError` — unknown acquirer → error

Deferred (P1, add if time):
- `TestPromptPayT3AcrossMonthBoundary`, `TestGlobalPayAcrossYearBoundary` — calendar math already tested implicitly via weekend skips; explicit boundaries only if time permits.

---

## Module B: Ingest — 3 Parsers (subagent-B, independent)

Path: `internal/ingest/`

Each parser: `func Parse{AcquirerName}(r io.Reader, sourceFile string) ([]SettlementRecord, error)`

Common requirements (checked once per parser):
- Column mapping by **header name** not position (P0 — codex flagged)
- UTF-8 BOM tolerated
- Empty required field → row-level error with row number
- Unknown columns ignored
- `gross - fee != net` in source → flag as discrepancy in record (don't reject)
- Amounts → `int64` satang

### B.1 — Thai Acquirer CSV (8 tests)

- [ ] `TestThaiCSVParsesValidRow` — full row → 1 record
- [ ] `TestThaiCSVParsesMultipleRows` — 5 rows → 5 records
- [ ] `TestThaiCSVRejectsMissingColumn` — missing `fee_amt` → error names missing column
- [ ] `TestThaiCSVRejectsEmptyFile` — 0 bytes → error (P0)
- [ ] `TestThaiCSVHeaderOnlyReturnsZeroRecords` — header only → `[]`, no error
- [ ] `TestThaiCSVColumnsParsedByHeader` — columns in different order still parse (P0)
- [ ] `TestThaiCSVTolerateUTF8BOM` — file starts with `﻿` → still parses (P1)
- [ ] `TestThaiCSVAttachesAcquirerAndSource` — record has `Acquirer=ThaiAcquirer`, `SourceFile=...`

### B.2 — GlobalPay CSV (6 tests)

- [ ] `TestGlobalCSVParsesValidRow` — valid row → 1 record
- [ ] `TestGlobalCSVMapsReferenceNumberToTransactionID` — `reference_number` → `TransactionID`
- [ ] `TestGlobalCSVParsesDDMMYYYY` — `"24/04/2026"` parses
- [ ] `TestGlobalCSVRejectsInvalidDate` — `"2026-04-24"` in DD/MM field → error
- [ ] `TestGlobalCSVFeeIsFixedPlusPercentage` — `10 THB + 2%` of 1000 = 30 → verify parsed net
- [ ] `TestGlobalCSVRejectsMissingColumn` — missing column → error

### B.3 — PromptPay JSON (7 tests)

- [ ] `TestPromptJSONParsesArray` — 3 objects → 3 records
- [ ] `TestPromptJSONParsesRFC3339WithBangkokOffset` — `"2026-04-24T10:00:00+07:00"` (P0)
- [ ] `TestPromptJSONParsesRFC3339UTC` — `"2026-04-24T03:00:00Z"` → same Bangkok date
- [ ] `TestPromptJSONRejectsMalformedJSON` — `"{"` → error with acquirer + file context
- [ ] `TestPromptJSONRejectsNullRequiredField` — `transaction_id: null` → error (P0)
- [ ] `TestPromptJSONRejectsEmptyObjectInArray` — `[{}]` → error (P0)
- [ ] `TestPromptJSONTieredFeeBoundary` — 4999.99 THB at 1.5%, 5000 THB at 1.8% (P0 — tier boundary)

### B.4 — Parser Dispatcher (1 test — INLINE in HTTP later)

- [ ] `TestParserDispatchByAcquirer` — maps acquirer name → parser function

**B total: 22 tests**

---

## Module C: Store + Reconcile (subagent-C, depends on Domain)

Path: `internal/store/` + `internal/reconcile/`

### C.1 — In-Memory Store (thread-safe, 8 tests)

Interface:
```go
type Store interface {
    SaveTransaction(Transaction) error
    SaveTransactions([]Transaction) error
    GetTransaction(id string) (Transaction, bool)
    ListTransactions(filter TxnFilter) []Transaction
    SaveSettlement(SettlementRecord) error
    FindSettlement(txnID string, acquirer Acquirer) (SettlementRecord, bool)
    ListSettlements(filter SettlementFilter) []SettlementRecord
}
```

Implementation uses `sync.RWMutex`.

Tests:
- [ ] `TestStoreSavesAndRetrievesTransaction`
- [ ] `TestStoreSavesAndRetrievesSettlement`
- [ ] `TestStoreDuplicateTransactionIsIdempotent` — same ID twice → 1 record (P0)
- [ ] `TestStoreDuplicateSettlementIsIdempotent` — same `(txnID, acquirer)` → 1 record (P0)
- [ ] `TestStoreConcurrentSaveAndRead` — 100 goroutines save + read, no race under `-race` (P0)
- [ ] `TestStoreFiltersTransactionsByDateRange`
- [ ] `TestStoreFiltersTransactionsByAcquirer`
- [ ] `TestStoreReturnedSlicesDoNotMutateInternal` — modifying returned slice doesn't affect store state (P1)

### C.2 — Reconciliation (9 tests)

Signature: `func Reconcile(txns []Transaction, settlements []SettlementRecord, asOf time.Time) ReconcileResult`

`ReconcileResult` contains `[]ReconciledTransaction` + `[]Discrepancy`.

Tests:
- [ ] `TestReconcileMarksMatchedAsSettled` — txn + matching settlement → settled
- [ ] `TestReconcileCarriesGrossFeeNet` — settled record carries amounts
- [ ] `TestReconcileMarksFutureExpectedAsPending` — no settlement + expected > asOf → pending
- [ ] `TestReconcileMarksPastDueAsOverdue` — no settlement + expected ≤ asOf → overdue
- [ ] `TestReconcileFlagsUnknownSettlementAsDiscrepancy` — settlement for unknown txn (P0)
- [ ] `TestReconcileFlagsAcquirerMismatch` — settlement acquirer ≠ txn acquirer → discrepancy (P0)
- [ ] `TestReconcileFlagsAmountMismatch` — gross ≠ txn amount → discrepancy (P0)
- [ ] `TestReconcileFlagsDuplicateSettlement` — same `(txnID, acquirer)` seen twice → discrepancy (P0)
- [ ] `TestReconcileAsOfBeforeTxnDoesNotMarkOverdue` — asOf < txnDate → pending (P1)

**C total: 17 tests**

---

## Module D: Query Service (main agent, after C)

Path: `internal/query/`

Signatures:
```go
ExpectedCashByAcquirer(date time.Time) QueryResult
UnsettledSince(days int, asOf time.Time) []ReconciledTransaction
FeesByAcquirer(month string) QueryResult  // "2026-04"
Overdue(asOf time.Time) []ReconciledTransaction
```

All dates use Bangkok TZ boundaries.

### Tests (7)

- [ ] `TestExpectedCashByAcquirerSumsNet` — multiple settlements on date → sum by acquirer
- [ ] `TestExpectedCashEmptyReturnsEmptyGroups` — no settlements that day → empty array, not null (P1)
- [ ] `TestUnsettledSinceFiltersByDays` — only past N days
- [ ] `TestUnsettledRejectsNegativeDays` — days=-1 → error (P0)
- [ ] `TestFeesByAcquirerForMonth` — sums fees per acquirer for YYYY-MM
- [ ] `TestFeesRejectsInvalidMonthFormat` — "2026/04" → error (P0)
- [ ] `TestFeesUsesBangkokMonthBoundaries` — txn on Apr 30 23:59 Bangkok → counts as April (P0)

---

## Module E: HTTP API (main agent, after D)

Path: `internal/api/` + `cmd/server/`

Endpoints:
```
POST /ingest/transactions                   — CSV body
POST /ingest/settlements/:acquirer          — CSV or JSON based on acquirer
GET  /queries/cashflow?date=YYYY-MM-DD      — default: tomorrow Bangkok
GET  /queries/unsettled?days=7
GET  /queries/fees?month=YYYY-MM
GET  /queries/overdue?as_of=YYYY-MM-DD      — default: now Bangkok
GET  /health
```

Error shape:
```json
{"error": "invalid_settlement_file", "message": "..."}
```

### Tests (8)

- [ ] `TestHealthReturnsOK` — GET /health → 200
- [ ] `TestIngestSettlementsDispatchesToParser` — POST ThaiAcquirer CSV → parsed + stored
- [ ] `TestIngestRejectsUnknownAcquirerPath` — `/ingest/settlements/FakeAcquirer` → 400 (P0)
- [ ] `TestIngestMalformedFileReturnsStructuredError` — broken CSV → 400 with shape (P0)
- [ ] `TestIngestSameFileTwiceIsIdempotent` — 2 identical POSTs → no duplicate records (P0)
- [ ] `TestQueriesRejectsBadDateParam` — `?date=foo` → 400 (P0)
- [ ] `TestQueriesUnsettledReturnsCorrectShape` — has `unsettled_transactions` array with expected fields
- [ ] `TestQueriesCashflowDefaultsToBangkokTomorrow` — no `?date=` → tomorrow Bangkok (P0)

---

## Module F: Test Data Generator (subagent-D, starts IMMEDIATELY — no Domain dep)

Path: `cmd/gen-testdata/` (standalone — writes raw files, doesn't import `internal/domain`)

Outputs to `project/data/`:
- `transactions.csv` — 300 txns
- `settlements/thai_acquirer.csv`
- `settlements/global_pay.csv`
- `settlements/promptpay.json`

### Tests (3)

- [ ] `TestGenerationIsDeterministicWithSeed` — same seed → identical output
- [ ] `TestGeneratesExactly300Transactions`
- [ ] `TestSettlementFilesCoverMixedStatus` — each settlement file has ≥1 settled + ≥1 missing (reconcile later flags pending/overdue)

Generation rules:
- Fixed seed `42`
- 300 txns over past 30 days
- 100 per acquirer
- Amounts 100-50000 THB
- 70% settled / 20% pending / 10% overdue
- Each settlement file: 50-100 entries (some txns have no settlement)

---

## Module G: Integration Test (main agent, Sprint 5)

Path: `cmd/server/integration_test.go`

### Tests (1 — most important)

- [ ] `TestEndToEndPipeline`
  - Load 6 hand-crafted txns (2 per acquirer)
  - Ingest 3 settlement files with 3 settled rows (1 per acquirer) + 1 overdue
  - Reconcile asOf fixed date
  - Assert: 3 settled / 1 pending / 2 overdue / fees grouped correctly / cashflow for date correct

---

## Stretch: Fee Anomaly Detection (only if time)

Path: `internal/anomaly/`

- [ ] `TestAnomalyFlagsFeeAboveThreshold` — expected 2.5%, actual 5% → flagged

Reuses fee rules from schedule/parser modules. Low cost if core done.

---

## Refined Counts

| Module | Tests | Priority |
|--------|-------|----------|
| 0 Domain | 5 | P0 |
| A Schedule | 10 | P0 |
| B Ingest (3 parsers + dispatcher) | 22 | P0 |
| C Store + Reconcile | 17 | P0 |
| D Query | 7 | P0 |
| E HTTP | 8 | P0 |
| F Generator | 3 | P0 (fixtures are scored) |
| G Integration | 1 | P0 (critical) |
| **Core Total** | **73** | |
| Stretch | 1 | P2 |

**Realistic cuts if behind schedule:**
- B.3 JSON tier boundary (merge into one `TestPromptJSONFeeByAmount` parametric test)
- C.1 `TestStoreFiltersByDateRange` / `ByAcquirer` — inline in query tests
- C.2 `TestReconcileAsOfBeforeTxnDoesNotMarkOverdue` — skip (rare)
- C.1 `TestStoreReturnedSlicesDoNotMutateInternal` — skip (defensive)
- E individual endpoint tests — cover via G integration

**Realistic achievable: ~55-60 tests + integration.**

---

## Parallelism Plan (Refined from Codex Feedback)

Key insight from codex: Domain does NOT need to be fully frozen before work starts. Schedule + data gen + parser drafting can begin immediately.

### Sprint 1 (T+20-35, 15 min)

```
T+20-21:  Agree on Domain interface stubs (Acquirer enum, Transaction+SettlementRecord shape) in ~1 min
T+21:     Spawn ALL 4 subagents in parallel:
  - subagent-A: Schedule (only needs Acquirer enum)
  - subagent-B: Parsers (needs Transaction + SettlementRecord shape — frozen above)
  - subagent-C: Store + Reconcile (needs all types — builds them as impl proceeds, converges with main at T+30)
  - subagent-D: Data generator (NO domain dep — writes raw files)
T+21-29:  Main agent completes Domain module in parallel (8 min)
T+29-35:  Main merges subagent output; resolves any type mismatches
```

**Write ownership (strict — codex insisted):**
- subagent-A: `internal/schedule/**` only
- subagent-B: `internal/ingest/**` only
- subagent-C: `internal/store/**` + `internal/reconcile/**` only
- subagent-D: `cmd/gen-testdata/**` + `data/**` only
- main: `internal/domain/**` + `internal/query/**` + `internal/api/**` + `cmd/server/**` + `go.mod`

**Ambiguity rule:** If any subagent needs a type not yet in Domain, they stub locally + flag in progress.md. Main agent reconciles at T+35 checkpoint.

### Sprints 2-4 (sequential — shared state)

Main agent owns Query + HTTP. Subagents pause. Reconcile + Query + HTTP share unsettled/status semantics — codex warned these should NOT fan out.

---

## Done Definition Per Module

Module is DONE when:
1. All listed tests exist and PASS under `go test -race ./internal/{module}/...`
2. No compilation errors in sibling modules
3. Committed with message `{module}: initial impl`
4. Module state row in `progress.md` updated to `X/Y done, passing`
