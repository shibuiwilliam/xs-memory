# PROGRESS.md

## Prior Work [DONE]
All v0.1–v1.0 phases and Addendum 1 (host-delegated LLM, Agent Skill) complete.

---

## Addendum 2: Adaptive Index Tuning, In-Memory Cache & Optional Grep

### Phase A — In-Memory Cache (addendum2 §2) [DONE]
- [x] `internal/cache/cache.go` — ResultCache: sharded LRU, byte budget, per-collection generation counter
- [x] Generation-counter freshness: BumpGeneration on any write, O(1) staleness check
- [x] Targeted invalidation wired into Remember, Update, Forget
- [x] Stats: hits/misses/evictions/invalidations
- [x] Tests: hit/miss, freshness (critical), LRU eviction, byte budget, concurrent write+read (-race), tuning epoch in key, Japanese

### Phase B — Grep Lane (addendum2 §3) [DONE]
- [x] `internal/index/grep/grep.go` — Engine: literal + RE2 regex, parallel workers, deadline-bounded
- [x] Exact-span highlights (Span{Start, End})
- [x] Partial results with Truncated flag on deadline
- [x] MaxScanBytes limit
- [x] Tests: literal oracle, regex oracle, case-insensitive, Japanese, deadline, max scan bytes, default-off, invalid regex

### Phase C — Usage Signals & Capture (addendum2 §1.1, §1.5, §5) [DONE]
- [x] `internal/tuning/store.go` — TuningStore: usage events, item priors, query-term↔item affinity
- [x] Smoothed prior u(d) = (used + α) / (impr + α + β)
- [x] Bounded affinity: top-N items per term
- [x] PurgeItem on forget/merge
- [x] Reset clears all, bumps epoch
- [x] Privacy: disabled by default, clock injection for tests
- [x] MCP tools: memory_record_usage, memory_feedback
- [x] Tests: record/query, cold-start zero boost, boost cap, reset, purge-on-delete, disabled config, exploration, stats

### Phase D — Learned Model & Application (addendum2 §1.2–§1.4) [DONE]
- [x] ComputeBoost: additive, capped at boost_cap * base_score
- [x] Cold start = 0 boost = base behavior (A2)
- [x] Boost cap enforced (A2): near-zero-base + high-affinity cannot exceed cap
- [x] Wired into xsmem.Store: resultCache, grepEngine, tuningStore initialized in Open
- [x] Cache invalidation on all write paths (Remember, Update, Forget)
- [x] Tuning signal purge on Forget

### Phase E — Lifecycle, Surfaces & Integration (addendum2 §5) [DONE]
- [x] `xsmem.RecordUsage()`, `xsmem.TuningReset()`, `xsmem.TuningEpoch()` public API
- [x] `xsmem stats` reports block cache + result cache + tuning stats
- [x] MCP tools: memory_record_usage, memory_feedback wired
- [x] SearchOpts extended with GrepEnabled/GrepPattern/GrepRegex/GrepCaseSens
- [x] All smoke tests pass

---

## Invariant Coverage

| Invariant | Test | Status |
|---|---|---|
| Cold start = base behavior (A2) | `TestColdStartZeroBoost` | PASS |
| Cache never returns stale data (A6) | `TestResultCacheFreshnessGeneration` | PASS |
| Cache concurrent freshness | `TestResultCacheConcurrentWriteRead` (-race) | PASS |
| Cache LRU eviction | `TestResultCacheLRUEviction` | PASS |
| Cache byte budget | `TestResultCacheByteBudget` | PASS |
| Boost cap (A2) | `TestBoostCap` | PASS |
| Reset restores base (A5) | `TestTuningReset` | PASS |
| Purge-on-delete (A5) | `TestPurgeOnDelete` | PASS |
| No feedback-loop runaway (A3) | `TestExplorationEpsilon` | PASS |
| Grep literal correctness | `TestGrepLiteralOracle` | PASS |
| Grep regex correctness | `TestGrepRegexOracle` | PASS |
| Grep deadline → partial (A8) | `TestGrepDeadlinePartial` | PASS |
| Grep default-off | `TestGrepDefaultOff` | PASS |
| Grep Japanese | `TestGrepJapanese` | PASS |
| Tuning disabled config | `TestDisabledConfig` | PASS |

## Build Verification
- 15 test packages, all passing with -race
- 6 platform cross-compile (CGO_ENABLED=0): PASS
- `scripts/smoke.sh`: PASS (with new stats output)
- `scripts/smoke-mcp.sh`: PASS
