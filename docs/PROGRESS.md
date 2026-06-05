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

---

## Addendum 3: Optional Metrics & Observability

### Phase A — Metrics registry & no-op core (addendum3 §1, §2, M1) [DONE]
- [x] `internal/metrics/recorder.go` — `Recorder` interface, `NopRecorder` (disabled), `LiveRecorder` (enabled)
- [x] `internal/metrics/histogram.go` — fixed 9-bucket latency histogram, no timestamps (M3)
- [x] `internal/metrics/topk.go` — bounded, decayed top-K term-frequency table (M7)
- [x] `internal/metrics/snapshot.go` — `Snapshot` struct with `ComputeModeDistribution()` (M4)
- [x] `xsmem/options.go` — `WithMetrics(cfg)` option, `MetricsConfig`/`MetricsSnapshot` re-exports
- [x] `xsmem/xsmem.go` — Store holds `Recorder`; `Search()` records metrics; `MetricsSnapshot()`/`MetricsReset()`/`MetricsEnabled()` public API
- [x] Zero overhead proven: `BenchmarkNopRecorderSearch` = 0 B/op, 0 allocs/op (M1)
- [x] Tests: NopRecorder interface, disabled=Nop, enabled=Live, search counts, hit rate + underfilled, mode distribution (incl. grep), latency off by default, latency buckets when enabled, no precise timing (M3), reset, snapshot aggregation, top-K bounded (M7), top-K decay, top-K reset
- [x] Integration: `TestMetricsDisabledByDefault`, `TestMetricsEnabledRecordsCounts`, `TestMetricsHitRate`, `TestMetricsReset`

### Phase B — Search aggregates (addendum3 §1.1, §1.3, §1.4, §1.5) [DONE]
(Delivered in Phase A — search count, hit rate, underfilled, latency, mode distribution all implemented and tested)

### Phase C — Keyword metrics (addendum3 §1.2, M2) [DONE]
(Delivered in Phase A — TopKTerms with bounded, decayed, hash-only default implemented and tested)

### Phase D — Index-level structural stats (addendum3 §1.6, M5) [DONE]
- [x] `internal/metrics/structural.go` — `StructuralStats` struct
- [x] `internal/index/fts/fts.go` — added `TermCount()` method
- [x] `internal/index/vector/vector.go` — added `Quantized()` method
- [x] `xsmem/xsmem.go` — `StructuralStats()` with generation-counter caching (M5)
- [x] Stats struct extended with `Structural` + `MetricsEnabled`
- [x] Tests: oracle on seeded store, cached-by-generation (reuse between writes, recompute after write), vector index, graph edges

### Phase E — Persistence, lifecycle & exposure (addendum3 §2, §3) [DONE]
- [x] `cmd/xsmem/metrics_cmd.go` — `xsmem metrics [--json] [--collection] [--reset]`
- [x] `cmd/xsmem/stats.go` — extended with FTS/vector/graph structural stats + metrics flag
- [x] `interfaces/mcp/mcp.go` — `memory_metrics` MCP tool
- [x] `scripts/smoke-metrics.sh` — end-to-end smoke test
- [x] Tests: `xsmem stats --json` validates as JSON; `xsmem metrics --reset` clears; privacy check passes

## Addendum 3 Invariant Coverage

| Invariant | Test | Status |
|---|---|---|
| Disabled = zero overhead (M1) | `BenchmarkNopRecorderSearch` (0 B/op, 0 allocs/op) | PASS |
| Privacy / no raw queries (M2) | `TestTopKCounting` (hashed tokens only), smoke privacy check | PASS |
| No timing (M3) | `TestLatencyBucketsOffByDefault`, `TestHistogramNoPreciseTiming` | PASS |
| Counts/hit-rate correct | `TestLiveRecorderSearchCounts`, `TestHitRateAndUnderfilled` | PASS |
| Mode distribution sums + grep (M4) | `TestModeDistribution` | PASS |
| Structural stats cached by generation (M5) | `TestStructuralStatsCachedByGeneration` | PASS |
| Bounded top-K (M7) | `TestTopKBounded` | PASS |
| Exposure: metrics JSON | `smoke-metrics.sh` (stats --json validates) | PASS |
| Exposure: memory_metrics MCP | `TestServerCreation` (tool registered) | PASS |
| Reset clears | `TestReset`, `TestMetricsReset` | PASS |

## Build Verification
- 15 test packages, all passing with -race
- 6 platform cross-compile (CGO_ENABLED=0): PASS
- `scripts/smoke.sh`: PASS (with new stats output)
- `scripts/smoke-mcp.sh`: PASS
