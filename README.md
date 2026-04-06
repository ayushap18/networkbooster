# NetworkBooster

A system-wide network speed optimizer and bandwidth testing tool for macOS. Boosts download and upload speeds across all applications by tuning TCP kernel parameters, optimizing DNS, and providing real-time bandwidth testing.

## What It Does

### Network Optimizer (`optimize`)
Applies system-wide network optimizations that boost speed for **every app** on your machine:

| Optimization | Before | After | Impact |
|---|---|---|---|
| **DNS** | ISP default | Fastest (Cloudflare/Google/Quad9) | Faster page loads, lower latency |
| **TCP Send/Receive Buffers** | 128 KB | 256 KB | More data in flight |
| **Auto Buffer Max** | 4 MB | 8 MB | Better throughput on fast links |
| **TCP MSS** | 512 | 1460 | ~3x fewer packets per transfer |
| **TCP Window Scale** | 3 | 8 | Much larger TCP window |
| **Delayed ACK** | Enabled | Disabled | Instant acknowledgements |
| **DNS Cache** | Stale entries | Flushed | Fresh lookups |

### Bandwidth Tester (`start`)
Saturates your connection using parallel downloads from the nearest servers to measure real throughput:
- Auto-discovers nearest servers via Ookla (thousands worldwide, including local ISPs)
- Latency probes all candidates, uses the 5 fastest
- Real-time TUI with speed gauges, connection stats, and session history
- Adaptive connection scaling (auto-adjusts worker count based on throughput)

## Installation

### From Source

```bash
# Requires Go 1.21+
git clone https://github.com/ayushap18/networkbooster.git
cd networkbooster
go build -o bin/networkbooster ./cmd/cli/
```

### Quick Start

```bash
# Optimize your network (requires sudo for system settings)
sudo ./bin/networkbooster optimize

# Run a speed test
./bin/networkbooster start

# Undo all optimizations
sudo ./bin/networkbooster reset
```

## Commands

### `optimize` — Boost System Network Speed
```bash
sudo ./bin/networkbooster optimize
```
Tests DNS resolvers, tunes TCP kernel parameters, and flushes DNS cache. All changes are system-wide and benefit every application.

**Example output:**
```
NetworkBooster — System Network Optimizer
==================================================

Testing DNS servers...

  Optimize DNS                         OK      ISP Default -> Cloudflare (1.1.1.1, 1.0.0.1) — 12ms
  Disable TCP Delayed ACK              OK      3 -> 0
  Increase TCP Send Buffer             OK      131072 -> 262144
  Increase TCP Receive Buffer          OK      131072 -> 262144
  Increase Auto Receive Buffer Max     OK      4194304 -> 8388608
  Increase Auto Send Buffer Max        OK      4194304 -> 8388608
  Optimize TCP MSS                     OK      512 -> 1460
  Increase TCP Window Scale            OK      3 -> 8
  Flush DNS Cache                      OK       -> cache flushed

Done! 9/9 optimizations applied.
```

### `reset` — Restore Defaults
```bash
sudo ./bin/networkbooster reset
```
Reverts all TCP settings to macOS defaults and restores DNS to automatic (DHCP).

### `start` — Bandwidth Test with Live TUI
```bash
./bin/networkbooster start                    # Default (medium profile, 16 connections)
./bin/networkbooster start --profile light    # 4 connections, gentle
./bin/networkbooster start --profile full     # 64 connections, max saturation
./bin/networkbooster start --adaptive         # Auto-adjust connections
./bin/networkbooster start --mode upload      # Test upload speed
./bin/networkbooster start --mode bidirectional  # Test both
```

### `schedule` — Scheduled Runs
```bash
./bin/networkbooster schedule
```
Runs speed tests on a schedule defined in `~/.networkbooster/config.yaml`.

### `history` — Session History
```bash
./bin/networkbooster history
```

### `daemon install/uninstall` — Background Service
```bash
sudo ./bin/networkbooster daemon install     # macOS launchd / Linux systemd
sudo ./bin/networkbooster daemon uninstall
```

### `config` — View Settings
```bash
./bin/networkbooster config
```

## Configuration

Config file: `~/.networkbooster/config.yaml`

```yaml
mode: download          # download | upload | bidirectional
profile: medium         # light | medium | full
connections: 16

safety:
  max_download_mbps: 0        # 0 = unlimited
  max_upload_mbps: 0
  daily_data_limit_gb: 50
  max_cpu_percent: 80
  max_temp_celsius: 85
  max_connections: 64

adaptive:
  enabled: false
  interval_secs: 5
  min_connections: 2
  max_connections: 0          # 0 = use safety max

schedule:
  - days: [mon, tue, wed, thu, fri]
    start: "02:00"
    end: "06:00"
    profile: full
```

## Architecture

```
networkbooster/
├── cmd/cli/              # CLI entry point (Cobra)
├── core/
│   ├── engine/           # Bandwidth engine, workers, adaptive scaler
│   ├── sources/          # Pluggable sources (Speedtest/Ookla, CDN, self-hosted)
│   ├── metrics/          # Real-time speed tracking, SQLite history
│   ├── optimizer/        # System-wide network optimizer (TCP, DNS)
│   ├── scheduler/        # Time-based scheduling
│   ├── safety/           # Bandwidth caps, CPU/temp monitoring
│   └── daemon/           # OS service (launchd/systemd)
├── ui/tui/               # Bubbletea terminal UI
├── config/               # YAML config handling
└── internal/testutil/    # Shared test helpers
```

## How the Optimizer Works

The optimizer modifies macOS kernel parameters via `sysctl` that control how TCP connections behave:

- **Bigger buffers** = more data can be "in flight" between your machine and remote servers, which is critical for high-bandwidth connections
- **Larger TCP windows** = the connection can scale up to use more of your available bandwidth
- **Full MSS (1460 vs 512)** = each TCP segment carries ~3x more data, reducing overhead
- **No delayed ACK** = the remote server gets acknowledgements faster, so it sends more data sooner
- **Fastest DNS** = domain name lookups resolve in milliseconds instead of the typical 50-100ms of ISP DNS

These are the same tunings that Linux power users apply via `sysctl.conf`. The changes persist until reboot (or until you run `networkbooster reset`).

## Platform Support

| Feature | macOS | Linux |
|---|---|---|
| Network Optimizer | Full support | Planned |
| Bandwidth Tester | Full support | Full support |
| Daemon Service | launchd | systemd |
| Speed Test Sources | Ookla + CDN | Ookla + CDN |

## License

MIT
