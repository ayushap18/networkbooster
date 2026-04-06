# Phase 5: Adaptive Scaling + Speedtest Source — Design Spec

## Overview

Add the Ookla speedtest source (with bundled fallback) and adaptive connection scaling that dynamically adjusts worker count based on throughput measurements. These two features make the engine smarter — more server options and self-tuning connection management.

## Speedtest Source

New `Source` implementation for Ookla-compatible speedtest servers.

### Server Discovery

Two-tier approach:

1. **Live discovery (primary):** Fetch the public Ookla server list XML endpoint (the same endpoint open-source speedtest-cli tools use). Parse server entries for ID, name, URL, lat/lng, and sponsor.
2. **Bundled fallback:** A hardcoded list of ~20 well-known speedtest servers across major regions. Used when live fetch fails (network issues, endpoint changes).

After discovery, servers are ranked by:
- Geographic proximity (haversine distance from a rough client location via IP geolocation or system timezone)
- Latency probe (HTTP HEAD to each candidate, pick top 5-10 by lowest RTT)

### Download

HTTP GET to the server's download test endpoint. Speedtest servers serve configurable random payload sizes. Use chunked reads (same pattern as CDN source), data discarded after measurement.

### Upload

HTTP POST random-filled data to the server's upload endpoint. Speedtest servers accept multipart form uploads of configurable sizes.

### Latency

HTTP HEAD request to server URL, measure round-trip time.

### Files

- `core/sources/speedtest.go` — SpeedtestSource implementation
- `core/sources/speedtest_test.go` — tests with bundled server fallback, mock HTTP for discovery

## Adaptive Scaler

A new component that monitors throughput and adjusts engine connection count at runtime.

### Algorithm: Gradual with Backoff

```
state: current_connections, throughput_history[3], stall_count

every interval (default 5s):
  sample = current throughput (rolling average from collector)
  append sample to throughput_history

  if len(history) < 3: continue  // need baseline

  delta = (latest - oldest) / oldest

  if delta >= 0.05:       // 5% improvement
    stall_count = 0
    add 1-2 connections (up to max)
  else if delta < 0.01:   // flat or declining
    stall_count++
    if stall_count >= 3:
      remove 1 connection (down to min)
      stall_count = 0
  else:
    stall_count = 0       // marginal improvement, hold steady
```

### Constraints

- Floor: `adaptive_min_connections` (default 2)
- Ceiling: min(`adaptive_max_connections`, `safety.max_connections`) — whichever is lower
- If safety monitor pauses the engine, scaler resets its history on resume
- Scaler is disabled when engine is paused

### Files

- `core/engine/scaler.go` — Scaler struct, algorithm, goroutine loop
- `core/engine/scaler_test.go` — tests for ramp-up, plateau detection, backoff, safety interaction

## Engine Changes

The engine currently creates a fixed set of workers at Start() and tears them all down at Stop(). To support adaptive scaling, it needs fine-grained worker management.

### New Methods

- `SetConnections(n int)` — Adjusts worker count at runtime. If n > current, launches additional workers against discovered servers (round-robin). If n < current, cancels excess workers (LIFO — most recently added first). Thread-safe.
- `ConnectionCount() int` — Returns current active worker count.

Each worker gets its own cancellable context (child of the engine's main context) so individual workers can be stopped without affecting others.

### Implementation

Workers tracked in a slice. Each entry holds the worker, its cancel func, and a done channel. `SetConnections` appends or pops from this slice.

### Files

- Modify `core/engine/engine.go` — per-worker context, SetConnections, ConnectionCount

## Config Changes

New fields in `Config`:

```yaml
adaptive: true                # enable adaptive scaling (default: false)
adaptive_interval_secs: 5     # evaluation interval
adaptive_min_connections: 2   # connection floor
adaptive_max_connections: 0   # 0 = defer to safety.max_connections
```

### Files

- Modify `config/config.go` — add AdaptiveConfig struct and fields

## CLI Integration

- `start` command reads adaptive config, starts scaler alongside engine when enabled
- New flag: `--adaptive` / `-a` to enable adaptive mode from CLI
- TUI already displays connection count — dynamic changes reflected automatically
- Session summary includes peak connection count reached

### Files

- Modify `cmd/cli/main.go` — wire adaptive config and scaler

## Testing Strategy

- Speedtest source: mock HTTP server for discovery XML and download/upload endpoints
- Scaler: mock collector returning controlled throughput values, verify connection adjustments
- Engine SetConnections: verify workers added/removed correctly, no races
- Integration: build succeeds, `go vet` clean, all tests pass
