# NetworkBooster — Design Spec

## Overview

A Go-based bandwidth saturation tool that continuously maximizes download/upload throughput by opening parallel connections to multiple server sources — the same technique speed testers use, but running continuously rather than as a one-off measurement.

Two interfaces: a CLI with TUI (bubbletea) for lightweight/low-power devices, and a desktop GUI (Wails) for regular machines.

## Architecture

**Approach:** Monorepo with shared core. One Go module, shared `core` library, CLI and GUI as separate build targets.

```
networkbooster/
├── cmd/
│   ├── cli/              # CLI entry point
│   └── gui/              # Wails GUI entry point
├── core/
│   ├── engine/           # Bandwidth engine — connection management, saturation
│   ├── sources/          # Pluggable sources (speedtest, CDN, self-hosted, P2P)
│   ├── metrics/          # Speed tracking, stats, history
│   ├── scheduler/        # Scheduled runs, profiles, on-demand control
│   └── safety/           # Bandwidth caps, data limits, CPU/temp monitoring
├── ui/
│   └── frontend/         # Wails web frontend (HTML/CSS/JS)
├── config/               # YAML config file handling
├── pkg/
│   └── netutil/          # Low-level networking helpers
├── go.mod
└── Makefile
```

## Bandwidth Engine

The heart of the system. Opens N parallel TCP connections to source servers and streams data to saturate the pipe.

### How It Works

1. **Connection Pool** — Opens N concurrent connections (default 8, auto-tunable up to 64) spread across available sources.
2. **Download Mode** — Each connection HTTP GETs large chunks (1-10MB). Data read and discarded. Aggregate throughput = real download speed.
3. **Upload Mode** — Each connection HTTP POSTs random/zero-filled data to servers accepting uploads.
4. **Bidirectional Mode** — Both simultaneously for full duplex saturation.
5. **Adaptive Scaling** — Starts few connections, measures throughput, adds more until speed plateaus or safety limits hit. Backs off when other apps need bandwidth.

### Engine Interface

```go
type Engine interface {
    Start(ctx context.Context, mode Mode) error
    Stop() error
    Status() EngineStatus
    SetProfile(profile Profile)
    OnMetrics(fn MetricsCallback)
}

type Mode int
const (
    ModeDownload Mode = iota
    ModeUpload
    ModeBidirectional
)
```

### Connection Lifecycle

- Engine picks sources from registry (round-robin + weighted by speed)
- Opens HTTP/2 or HTTP/1.1 connections with keep-alive
- Continuously streams data in a loop until stopped
- Reconnects on drop (same or different source)
- Metrics collector samples throughput every 100ms

## Source System

Pluggable source types behind a common interface:

```go
type Source interface {
    Name() string
    Type() SourceType
    Discover() ([]Server, error)
    Download(ctx context.Context, server Server) (io.ReadCloser, error)
    Upload(ctx context.Context, server Server, r io.Reader) error
    Latency(server Server) (time.Duration, error)
}
```

### Source Types

| Source | Discovery | Download | Upload |
|--------|-----------|----------|--------|
| **Speedtest (Ookla)** | Fetches server list from Ookla API, picks closest/fastest | HTTP GET test payloads | HTTP POST to upload endpoints |
| **CDN mirrors** | Bundled list of known large test files (Ubuntu ISOs, Hetzner, OVH, etc.) | HTTP GET range requests | Not supported (read-only) |
| **Self-hosted** | User configures server address in config | HTTP GET from user's server | HTTP POST to user's server |
| **P2P** | mDNS/UDP broadcast on LAN + optional relay for WAN | Direct TCP stream between peers | Direct TCP stream between peers |

### Source Selection Strategy

- On startup, discover all available servers across all source types
- Quick latency check on each
- Rank by latency + throughput (small probe test)
- Distribute connections across top-N servers for redundancy
- Re-evaluate every 60s, drop slow servers, promote faster ones

### P2P Details

- Peers exchange random data — just filling the pipe, no actual file transfer
- LAN discovery via mDNS/UDP broadcast
- WAN discovery via optional lightweight relay/signaling server
- Peer identity via keys stored in `~/.networkbooster/keys/`

## Metrics & Display

### Metrics Collected

Sampled every 100ms:

| Metric | Description |
|--------|-------------|
| Current download/upload speed | Rolling 1-second average (Mbps) |
| Per-server speed | Throughput breakdown by each connected server |
| Active connections | Count per source type |
| Latency | RTT to each server |
| Total data transferred | Session + all-time (up and down separately) |
| Bandwidth utilization % | Current speed vs detected max capacity |
| Peak / Average / Min speed | Per session and historical |
| Per-interface breakdown | Stats per network interface (Wi-Fi vs Ethernet) |

### History Storage

- SQLite database in `~/.networkbooster/history.db`
- Per-second aggregates, session summaries, daily rollups
- Configurable retention (default 30 days)

### CLI Display (bubbletea TUI)

- Live-updating terminal UI
- Speed gauges as bar charts, per-server table, running totals
- Compact mode for low-power devices: `DOWN 142.3 Mbps UP 38.1 Mbps | 12 conns | 2.4 GB`

### GUI Display (Wails)

- Real-time line charts for speed over time
- Speedometer/gauge widgets for current speed
- Server list with per-server stats
- Tabbed views: Live, History, Servers, Settings
- System tray icon with quick stats on hover

## Safety System

All safety checks run every 500ms and can throttle or pause the engine.

| Feature | Behavior |
|---|---|
| **Bandwidth cap** | User sets max Mbps. Engine reduces connections or throttles when approaching limit. |
| **Data usage limit** | Daily/monthly GB cap. Tracks in SQLite. Warns at 80%, pauses at 100%. |
| **Network priority** | Monitors system network activity via OS APIs. Backs off when other apps increase usage. Priority levels: aggressive/balanced/polite. |
| **CPU monitoring** | Checks every 2s. If sustained above threshold (default 80%), reduces connections. |
| **Temperature monitoring** | Reads thermal sensors (sysctl/macOS, /sys/class/thermal/Linux, WMI/Windows). Backs off at warning, pauses at critical. |
| **Connection limit** | Hard cap on max concurrent connections (default 64). |

### Safety Config

```yaml
safety:
  max_download_mbps: 0        # 0 = unlimited
  max_upload_mbps: 0
  daily_data_limit_gb: 50
  monthly_data_limit_gb: 500
  priority_mode: balanced      # aggressive | balanced | polite
  max_cpu_percent: 80
  max_temp_celsius: 85
  max_connections: 64
```

### Graceful Behavior

- Never hard-kills connections — always drains gracefully
- Logs every throttle/pause event with reason
- GUI shows safety indicator (green/yellow/red)
- CLI prints warnings when limits hit

## Scheduler & Profiles

### Profiles

| Profile | Connections | Behavior |
|---|---|---|
| **Light** | 4 | Polite priority, low CPU target. Background use. |
| **Medium** | 16 | Balanced priority. Default. |
| **Full** | 64 | Aggressive, all sources, max saturation. |
| **Custom** | User-defined | Override any parameter via config. |

### Control Modes

1. **Always on** — Registered as OS service (systemd/launchd/Windows Service), runs chosen profile until stopped.
2. **Scheduled** — Time windows in config:
   ```yaml
   schedule:
     - days: [mon, tue, wed, thu, fri]
       start: "20:00"
       end: "23:00"
       profile: full
     - days: [sat, sun]
       start: "00:00"
       end: "06:00"
       profile: medium
   ```
3. **On-demand** — Manual start/stop via CLI or GUI, pick a profile.

### CLI Commands

```
networkbooster start                  # on-demand, default profile
networkbooster start --profile full   # on-demand, specific profile
networkbooster stop                   # stop running engine
networkbooster status                 # show current stats
networkbooster schedule               # start in scheduled mode
networkbooster daemon install         # install as OS service
networkbooster daemon uninstall       # remove OS service
networkbooster config                 # open/edit config
networkbooster history                # show historical stats
```

## Distribution & Cross-Platform

### Build Targets

| Platform | CLI | GUI | Install Methods |
|---|---|---|---|
| macOS (arm64, amd64) | Yes | Yes | Homebrew, direct download |
| Linux (arm64, amd64, armv7) | Yes | Yes | apt, snap, direct download |
| Windows (amd64) | Yes | Yes | scoop, direct download |

armv7 for low-power devices (Raspberry Pi) — CLI-only.

### Build & Release

- Makefile with targets: `make cli`, `make gui`, `make all`, `make cross`
- GoReleaser for automated cross-compilation and packaging
- GitHub Releases for direct binary downloads
- Package manager formulae/manifests in repo

### Binary Sizes (Estimated)

- CLI: ~10-15MB
- GUI: ~20-30MB (Wails bundles webview)

### Config & Data Locations

- Config: `~/.networkbooster/config.yaml`
- Database: `~/.networkbooster/history.db`
- Logs: `~/.networkbooster/logs/`
- P2P keys: `~/.networkbooster/keys/`
