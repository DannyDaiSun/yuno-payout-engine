# Behaviors — Bangkok Settlement Maze

Spec → test names. In-memory store. Money = `int64` minor units (satang). All times in **Bangkok timezone (UTC+7)**.

Every behavior below is backed by a passing Go unit test (`go test -race -count=1 ./...`).

---

## Disambiguation (resolved before parallel work)

1. **3 acquirers:** ThaiAcquirer (daily CSV), GlobalPay (Tue+Fri CSV), PromptPayProcessor (T+3 JSON)
2. **GlobalPay same-day policy:** transaction on Tue 23:59 → settles **next Friday** (not same Tue — same-day ineligible). Rationale: settlement batches close at midnight.
3. **Dates in spec (YYYY-MM-DD etc) are Bangkok-local dates.** Normalize to Bangkok midnight internally.
4. **Public holidays:** ignored (future work). Only Sat+Sun treated as non-business days.
5. **Refunds / negative amounts:** out of scope. Parser rejects negatives.
6. **Duplicate `(txn_id, acquirer)` settlements:** first-row-wins at store layer; reconcile flags as `DiscrepancyDuplicateSettlement`.
7. **Currency:** always THB. Parser rejects non-THB.
8. **Same-day expected settlement:** stays `pending` (settlement may still arrive today). Only past Bangkok days are `overdue`.

---

## Module 0: Domain (`internal/domain/`)

Types: `Acquirer`, `PaymentMethod`, `SettlementStatus`, `Transaction`, `SettlementRecord`, `ReconciledTransaction`, `Discrepancy`, `DiscrepancyReason`.
Helpers: `ParseMinorUnits`, `FormatMinorUnits`, `BangkokTZ`, `BangkokMidnight`, `IsWeekend`, `NextBusinessDay`, `AddBusinessDays`.

- [x] `TestParseAmountToMinorUnits` — `"1000.25"` → `100025`
- [x] `TestParseAmountRejectsNegative` — `"-5.00"` → error
- [x] `TestParseAmountRejectsMoreThanTwoDecimals` — `"100.001"` → error
- [x] `TestFormatMinorUnits` — `100025` → `"1000.25"`
- [x] `TestBangkokMidnightNormalizes` — UTC instant → same Bangkok day at 00:00 +07:00

---

## Module A: Schedule Calculator (`internal/schedule/`)

Signature: `func ExpectedSettlementDate(acquirer Acquirer, txnDate time.Time) (time.Time, error)`

- [x] `TestThaiAcquirerNextBusinessDay` — Mon → Tue
- [x] `TestThaiAcquirerFridaySkipsToMonday` — Fri → Mon
- [x] `TestGlobalPayMondayGoesToTuesday` — Mon → Tue
- [x] `TestGlobalPayWednesdayGoesToFriday` — Wed → Fri
- [x] `TestGlobalPayTuesdaySkipsToFriday` — Tue (same-day ineligible) → Fri
- [x] `TestGlobalPayFridaySkipsToNextTuesday` — Fri → next Tue
- [x] `TestPromptPayT3Weekday` — Mon → Thu
- [x] `TestPromptPayT3AcrossWeekend` — Wed → next Mon
- [x] `TestPromptPayFridayTxnSettlesWednesday` — Fri +3 business → next Wed
- [x] `TestUnknownAcquirerReturnsError` — unknown → error

---

## Module B: Ingest — 3 Parsers (`internal/ingest/`)

Each parser: `func Parse{Acquirer}(r io.Reader, sourceFile string) ([]SettlementRecord, error)`.

Common guarantees: header-name mapping, Bangkok TZ, satang amounts, row-length safety.

### B.1 Thai Acquirer CSV (`thai_csv.go`)

- [x] `TestThaiCSVParsesValidRow`
- [x] `TestThaiCSVParsesMultipleRows`
- [x] `TestThaiCSVRejectsMissingColumn`
- [x] `TestThaiCSVRejectsEmptyFile`
- [x] `TestThaiCSVHeaderOnlyReturnsZeroRecords`
- [x] `TestThaiCSVColumnsParsedByHeader` — different column order still parses
- [x] `TestThaiCSVTolerateUTF8BOM`
- [x] `TestThaiCSVAttachesAcquirerAndSource`
- [x] `TestThaiCSVDatesAreBangkokTZ`
- [x] `TestGlobalCSVDoesNotPanicOnShortRow` — short row → error, not panic (lives in global file but covers both)

### B.2 GlobalPay CSV (`global_csv.go`)

- [x] `TestGlobalCSVParsesValidRow`
- [x] `TestGlobalCSVMapsReferenceNumberToTransactionID`
- [x] `TestGlobalCSVParsesDDMMYYYY`
- [x] `TestGlobalCSVRejectsInvalidDate`
- [x] `TestGlobalCSVFeeIsFixedPlusPercentage` — `10 + 2%` of 1000 = 30 captured
- [x] `TestGlobalCSVRejectsMissingColumn`

### B.3 PromptPay JSON (`prompt_json.go`)

- [x] `TestPromptJSONParsesValidRecord`
- [x] `TestPromptJSONParsesArray` — multi-element array
- [x] `TestPromptJSONParsesRFC3339WithBangkokOffset` — `+07:00`
- [x] `TestPromptJSONParsesRFC3339UTC` — `Z` → equivalent Bangkok day
- [x] `TestPromptJSONRejectsMalformedJSON`
- [x] `TestPromptJSONRejectsNullRequiredField`
- [x] `TestPromptJSONRejectsEmptyObjectInArray` — `[{}]` → error
- [x] `TestPromptJSONHandlesEmptyArray` — `[]` → 0 records, no error
- [x] `TestPromptJSONRejectsStringAmount` — strict-numeric (no quoted amounts)
- [x] `TestPromptJSONTieredFeeBoundary` — fee values at 4999.99 vs 5000.00 captured

---

## Module C: Store + Reconcile

### C.1 In-Memory Store (`internal/store/memory.go`)

Thread-safe (`sync.RWMutex`). Idempotent saves. Defensive copies on list.

- [x] `TestStoreSavesAndRetrievesTransaction`
- [x] `TestStoreSavesAndRetrievesSettlement`
- [x] `TestStoreDuplicateTransactionIsIdempotent`
- [x] `TestStoreDuplicateSettlementIsIdempotent`
- [x] `TestStoreConcurrentSaveAndRead` — 50 writers + 50 readers, race-clean
- [x] `TestStoreFindSettlementReturnsFalseWhenMissing`
- [x] `TestStoreReturnedSlicesDoNotMutateInternal`

### C.2 Reconciliation (`internal/reconcile/reconcile.go`)

Signature: `func Reconcile(txns, settlements, asOf) Result` returning `Reconciled []ReconciledTransaction` + `Discrepancies []Discrepancy`.

- [x] `TestReconcileMarksMatchedAsSettled`
- [x] `TestReconcileCarriesGrossFeeNet` — settled record carries amounts
- [x] `TestReconcileMarksFutureExpectedAsPending`
- [x] `TestReconcileMarksPastDueAsOverdue`
- [x] `TestReconcileAsOfBeforeTxnDoesNotMarkOverdue`
- [x] `TestReconcileFlagsUnknownSettlementAsDiscrepancy`
- [x] `TestReconcileFlagsAcquirerMismatch`
- [x] `TestReconcileFlagsAmountMismatch`
- [x] `TestReconcileFlagsDuplicateSettlement` — first-row-wins, single discrepancy with count
- [x] `TestReconcileFlagsTripleDuplicateOnce` — 3 dups → 1 discrepancy with `"3 times"` in detail

---

## Module D: Query Service (`internal/query/`)

- [x] `TestExpectedCashByAcquirerSumsNet`
- [x] `TestExpectedCashEmptyReturnsEmptyGroups`
- [x] `TestUnsettledSinceFiltersByDays`
- [x] `TestUnsettledRejectsNegativeDays`
- [x] `TestFeesByAcquirerForMonth`
- [x] `TestFeesRejectsInvalidMonthFormat`
- [x] `TestFeesUsesBangkokMonthBoundaries`
- [x] `TestSettledSinceReturnsMatchedTxns`

---

## Module E: HTTP API (`internal/api/`, `cmd/server/`)

Routes:
```
GET  /health
POST /ingest/transactions
POST /ingest/settlements/:acquirer
GET  /queries/cashflow?date=YYYY-MM-DD          (default: tomorrow Bangkok)
GET  /queries/unsettled?days=7&as_of=...
GET  /queries/fees?month=YYYY-MM
GET  /queries/overdue?as_of=...
GET  /queries/settled?days=7&as_of=...
GET  /queries/anomalies                          (stretch A)
GET  /queries/forecast?days=7&as_of=...          (stretch B)
```

Error shape: `{"error":"<code>","message":"<detail>"}`.

- [x] `TestHealthReturnsOK`
- [x] `TestIngestSettlementsDispatchesToParser` — Thai CSV + PromptPay JSON via correct parser
- [x] `TestIngestRejectsUnknownAcquirerPath`
- [x] `TestIngestMalformedFileReturnsStructuredError`
- [x] `TestIngestSameFileTwiceIsIdempotent`
- [x] `TestQueriesRejectsBadDateParam`
- [x] `TestQueriesUnsettledReturnsCorrectShape`
- [x] `TestQueriesCashflowDefaultsToBangkokTomorrow`
- [x] `TestQueriesSettledRejectsBadDays`
- [x] `TestQueriesForecastRejectsBadDays`

---

## Module F: Test Data Generator (`cmd/gen-testdata/`)

Deterministic `seed=42`. Produces 300 txns + 70 entries per settlement file. Settlement dates use `schedule.ExpectedSettlementDate` for correctness.

- [x] `TestGenerationIsDeterministicWithSeed`
- [x] `TestGeneratesExactly300Transactions`
- [x] `TestEachSettlementFileHas70Entries`
- [x] `TestPromptPayJSONEmitsNumericAmount`

---

## Module G: Integration (`cmd/server/integration_test.go`)

End-to-end pipeline test: ingest 3 settlement files → reconcile → query cashflow + fees + overdue.

- [x] `TestEndToEndPipeline`

---

## Stretch A: Fee Anomaly Detection (`internal/anomaly/`)

`func ExpectedFee(acquirer, grossMinor) (int64, error)` per acquirer rule.
`func Detect([]SettlementRecord) []Anomaly` flags actual vs expected fee deviations.

- [x] `TestExpectedFeeThaiAcquirer` — gross 1000 THB → fee 25 THB (2.5%)
- [x] `TestExpectedFeeGlobalPay` — gross 1000 THB → fee 30 THB (`10 + 2%`)
- [x] `TestExpectedFeePromptPayTiers` — boundary cases at 4999.99 / 5000 / 20000 / 20000.01 THB
- [x] `TestDetectFlagsAnomaly` — 1 normal + 1 with double fee → 1 critical anomaly

---

## Stretch B: Settlement Forecast (`internal/forecast/`)

`func Forecast(reconcile.Result, asOf, days) Result` — predicted daily cash inflow from pending txns over the next 1–14 days, grouped by acquirer.

- [x] `TestForecastIncludesFuturePending`
- [x] `TestForecastSkipsOverdueAndSettled`
- [x] `TestForecastSkipsBeyondWindow`

---

## Final Counts

| Module | Tests | Status |
|--------|-------|--------|
| 0 Domain | 5 | ✅ |
| A Schedule | 10 | ✅ |
| B Ingest (3 parsers) | 26 | ✅ |
| C Store + Reconcile | 17 | ✅ |
| D Query | 8 | ✅ |
| E HTTP | 10 | ✅ |
| F Generator | 4 | ✅ |
| G Integration | 1 | ✅ |
| Stretch A Anomaly | 4 | ✅ |
| Stretch B Forecast | 3 + 1 (HTTP) | ✅ |
| **Total** | **~89 unit tests** | **all green, race-clean** |

Run: `cd project && go test -race -count=1 ./...`
