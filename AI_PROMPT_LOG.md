# AI Prompt Log

Yuno explicitly invites use of AI tools and asks for a brief log. This file summarizes how AI was used during the 2-hour build.

## Tools Used

- **Claude Code (Anthropic, Opus 4.7)** — primary build agent. Drove the test-list TDD workflow, wrote the Go code, ran the tests, and shipped the docs you are reading.
- **Codex (OpenAI GPT-5.5)** — secondary review agent. Performed pre-implementation analysis, edge-case audits, correctness reviews after each sprint, and a final adversarial review.

## Workflow

**Subagent-driven test-list TDD.** Instead of micro RED -> GREEN -> REFACTOR cycles, work was batched per module:

1. Write the full unit-test list for a module (5 minutes)
2. Implement the module to satisfy the list (10-15 minutes)
3. `go test -race -count=1 ./internal/<module>/...` -> commit when green
4. Single refactor pass at module end if there is an obvious smell

The full plan lives in `behaviors.md`.

**Sprint structure (2 hour budget):**

- Sprint 0 (T+0 - T+20): challenge intake, scenario disambiguation, behaviors.md drafting
- Sprint 1 (T+20 - T+35): four parallel subagents on independent modules
  - subagent-A: `internal/schedule`
  - subagent-B: `internal/ingest` (3 parsers)
  - subagent-C: `internal/store` + `internal/reconcile`
  - subagent-D: `cmd/gen-testdata` (no domain dependency)
- Sprints 2-4: main agent owns query, HTTP, and integration test (sequential because shared state)
- Sprint 5: stretch goal (anomaly), final polish, docs

**Codex review checkpoints (inlined into workflow):**

| When | Codex task |
|------|------------|
| Pre-Sprint 1 | Edge-case audit of behaviors.md |
| End of Sprint 1 | Review ingest module across all 3 parsers |
| End of Sprint 2 | Review store thread-safety + reconcile discrepancy logic |
| End of Sprint 3 | Review query layer + HTTP API contract |
| End of Sprint 4 | Adversarial review of full pipeline |
| Sprint 5 | Codex generated the integration-test scenario |

**Write-ownership rule (strict):** each subagent owned exactly one directory tree. Domain types were frozen at T+20 and only the main agent edited `internal/domain`. This kept parallelism collision-free.

## Commit Trail

The build progression is visible in the git log (15+ commits across ~90 minutes of active coding):

```
643b302 tests: tier boundary + bangkok month + cashflow default + idempotent upload
163a4e0 anomaly: fee anomaly detection module + /queries/anomalies endpoint
6c0aa52 ingest: bangkok tz + row length checks + json amount handling
233e15f gen-testdata: business-day settlement dates + numeric JSON amounts
3c9bb28 server: fixture loader + end-to-end integration test
cad99e8 query+api: query service, HTTP handlers, transactions loader
be5d0bf gen-testdata: scale to 300 txns + 70 settlements per file
b6f9dc4 store+reconcile: edge tests + concurrency + discrepancy types
93501d0 ingest: edge case tests + impl fixes
ec7ff46 schedule: edge case tests + impl fixes
87b33af gen-testdata: smoke generator + fixture files
3774db3 store+reconcile: smoke tests + impl
73b866c ingest: smoke tests + 3 parsers
fcbead7 schedule: smoke tests + impl
152774e domain: types, money helpers, Bangkok timezone helpers
71087c0 initial skeleton: Go project with health endpoint and hello world test
```

Each commit corresponds to a module or sprint boundary. Tests are added in the same commit that implements them (test-list TDD), not afterward.

## What AI Was Not Used For

- No AI-generated business logic without a corresponding test.
- No AI-generated tests without inspecting the assertion to be sure it tests the intended behaviour.
- Decisions on scope cuts (public holidays, refunds, partial settlements) were made by the human and recorded in `behaviors.md` before any code was written.

## What Worked Well

- Splitting the work into four parallel subagents during Sprint 1 saved roughly 20 minutes of wall-clock time.
- Codex's pre-implementation edge-case audit caught the GlobalPay same-day ineligibility ambiguity before any code was written, avoiding rework.
- Test-list TDD per module (vs per behaviour) was a good fit for the 2-hour window: still test-first, but without the per-cycle context-switch overhead.

## What Was Cut for Time

- Settlement-prediction stretch goal (would have reused `ExpectedSettleDate` for a 7-14 day forecast).
- Public-holiday support beyond Sat/Sun.
- A few P1 tests listed in `behaviors.md` (parametric tier tests, returned-slice immutability) that were judged lower-value than the integration test.
