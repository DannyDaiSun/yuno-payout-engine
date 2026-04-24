# Test Cases

Inventory of every test in the suite, grouped by category. Run all with:

```bash
go test -race -count=1 ./...
```

Per-package run:

```bash
go test -race -count=1 ./internal/<pkg>
```

## Sanity (one or more per module — happy path)

| Test | Package | Purpose |
|------|---------|---------|
| `TestParseAmountToMinorUnits` | domain | `"1000.25"` -> `100025` |
| `TestFormatMinorUnits` | domain | `100025` -> `"1000.25"` |
| `TestBangkokMidnightNormalizes` | domain | Any `time.Time` -> same Bangkok day at 00:00 UTC+7 |
| `TestThaiAcquirerNextBusinessDay` | schedule | Mon txn -> Tue settlement |
| `TestThaiAcquirerFridaySkipsToMonday` | schedule | Fri -> Mon (skip weekend) |
| `TestGlobalPayMondayGoesToTuesday` | schedule | Mon -> Tue payout |
| `TestGlobalPayWednesdayGoesToFriday` | schedule | Wed -> Fri payout |
| `TestGlobalPayFridaySkipsToNextTuesday` | schedule | Fri -> next Tue |
| `TestPromptPayT3Weekday` | schedule | Mon -> Thu (T+3) |
| `TestPromptPayT3AcrossWeekend` | schedule | Wed -> next Mon (skip Sat/Sun) |
| `TestThaiCSVParsesValidRow` | ingest | Single Thai CSV row -> 1 record |
| `TestGlobalCSVParsesValidRow` | ingest | Single GlobalPay CSV row -> 1 record |
| `TestGlobalCSVParsesDDMMYYYY` | ingest | `"24/04/2026"` parses correctly |
| `TestGlobalCSVMapsReferenceNumberToTransactionID` | ingest | Field mapping correct |
| `TestPromptJSONParsesValidRecord` | ingest | One PromptPay JSON object -> 1 record |
| `TestStoreSavesAndRetrievesTransaction` | store | Save + Get round-trip |
| `TestStoreSavesAndRetrievesSettlement` | store | Save + FindSettlement round-trip |
| `TestReconcileMarksMatchedAsSettled` | reconcile | Txn + matching settlement -> settled |
| `TestReconcileMarksFutureExpectedAsPending` | reconcile | No settlement, expected > asOf -> pending |
| `TestReconcileMarksPastDueAsOverdue` | reconcile | No settlement, expected <= asOf -> overdue |
| `TestExpectedCashByAcquirerSumsNet` | query | Multiple settlements on date sum by acquirer |
| `TestFeesByAcquirerForMonth` | query | Per-acquirer fee totals for YYYY-MM |
| `TestExpectedFeeThaiAcquirer` | anomaly | Expected fee = 2.5% of gross |
| `TestExpectedFeeGlobalPay` | anomaly | Expected fee = 10 + 2% |
| `TestExpectedFeePromptPayTiers` | anomaly | Tiered: 1.5/1.8/2.2% based on gross |
| `TestDetectFlagsAnomaly` | anomaly | Settlement with off-rule fee -> flagged |
| `TestHealthReturnsOK` | api | `GET /health` -> 200 |
| `TestHello` | hello | Skeleton smoke test from initial scaffold |

## Regression (idempotency / ordering / boundary)

| Test | Package | Purpose |
|------|---------|---------|
| `TestStoreDuplicateTransactionIsIdempotent` | store | Same txn ID saved twice -> 1 record |
| `TestStoreDuplicateSettlementIsIdempotent` | store | Same `(txnID, acquirer)` saved twice -> 1 record |
| `TestStoreConcurrentSaveAndRead` | store | 100 goroutines under `-race` -> no race |
| `TestStoreFindSettlementReturnsFalseWhenMissing` | store | Lookup miss returns `false`, not panic |
| `TestPromptPayFridayTxnSettlesWednesday` | schedule | Fri txn + 3 business days -> next Wed (weekend skip) |
| `TestThaiCSVDatesAreBangkokTZ` | ingest | Parsed date is in Asia/Bangkok |
| `TestPromptPayJSONEmitsNumericAmount` | gen-testdata | Generator emits numeric (not string) JSON amount |
| `TestPromptJSONTieredFeeBoundary` | ingest | 4999.99 vs 5000.00 -> different tiers (boundary) |
| `TestFeesUsesBangkokMonthBoundaries` | query | Apr 30 23:59 Bangkok counts as April |
| `TestQueriesCashflowDefaultsToBangkokTomorrow` | api | Missing `?date=` -> Bangkok-tomorrow |
| `TestIngestSameFileTwiceIsIdempotent` | api | Two identical POSTs -> no duplicate records |
| `TestEachSettlementFileHas70Entries` | gen-testdata | Fixture file size guarantee |
| `TestGenerationIsDeterministicWithSeed` | gen-testdata | Same seed -> identical output |
| `TestGeneratesExactly300Transactions` | gen-testdata | Fixture cardinality |

## Integration (end-to-end pipeline)

| Test | Package | Purpose |
|------|---------|---------|
| `TestEndToEndPipeline` | cmd/server | 6 hand-crafted txns across all 3 acquirers; ingest 3 settlement files via HTTP; assert cashflow total, fees total, overdue count via the JSON API. Single test that proves the full pipeline is wired end-to-end. |

## Negative (error paths / malformed input)

| Test | Package | Purpose |
|------|---------|---------|
| `TestParseAmountRejectsNegative` | domain | `"-5.00"` -> error |
| `TestParseAmountRejectsMoreThanTwoDecimals` | domain | `"100.001"` -> error |
| `TestUnknownAcquirerReturnsError` | schedule | Unknown acquirer -> error |
| `TestThaiCSVRejectsMissingColumn` | ingest | Missing required column -> error names the column |
| `TestThaiCSVRejectsEmptyFile` | ingest | 0 bytes -> error |
| `TestThaiCSVHeaderOnlyReturnsZeroRecords` | ingest | Header only -> empty slice, no error |
| `TestGlobalCSVRejectsMissingColumn` | ingest | Missing column -> error |
| `TestGlobalCSVDoesNotPanicOnShortRow` | ingest | Truncated row -> error, not panic |
| `TestPromptJSONRejectsMalformedJSON` | ingest | `"{"` -> error with acquirer + file context |
| `TestPromptJSONRejectsNullRequiredField` | ingest | `transaction_id: null` -> error |
| `TestPromptJSONRejectsStringAmount` | ingest | Amount as string in JSON -> error |
| `TestPromptJSONHandlesEmptyArray` | ingest | `[]` -> empty slice, no error |
| `TestReconcileFlagsUnknownSettlementAsDiscrepancy` | reconcile | Settlement for unknown txn -> discrepancy |
| `TestReconcileFlagsAcquirerMismatch` | reconcile | Txn acquirer != settlement acquirer -> discrepancy |
| `TestReconcileFlagsAmountMismatch` | reconcile | Gross != txn amount -> discrepancy |
| `TestUnsettledRejectsNegativeDays` | query | `days=-1` -> error |
| `TestFeesRejectsInvalidMonthFormat` | query | `"2026/04"` -> error |
| `TestIngestRejectsUnknownAcquirerPath` | api | `/ingest/settlements/FakeAcquirer` -> 400 |
| `TestQueriesRejectsBadDateParam` | api | `?date=foo` -> 400 |
| `TestGlobalPayTuesdaySkipsToFriday` | schedule | Tue txn (same-day ineligible) -> Fri (documented edge) |

## Coverage Notes

- ~60 distinct test functions across 12 packages
- All tests pass under `go test -race -count=1 ./...`
- Single integration test exercises HTTP -> parsers -> store -> reconcile -> query in one flow
- Tests that span >1 module (e.g. `TestExpectedFeePromptPayTiers`) live with the consumer package, not the producer
