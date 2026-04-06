# Phase 4: Scheduler + Profiles + Daemon — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add profile presets (light/medium/full/custom), time-based scheduling, and OS service registration (launchd/systemd) so the booster can run as a daemon.

**Architecture:** New package `core/scheduler` with schedule parsing and time-window evaluation. Config extended with schedule entries. CLI gains `schedule`, `daemon install`, `daemon uninstall` commands.

**Tech Stack:** Go stdlib (time, os/exec), launchd plist generation (macOS), systemd unit generation (Linux)

---

### Task 1: Profile System

**Files:**
- Create: `core/engine/profile.go`
- Create: `core/engine/profile_test.go`

Profiles map to engine Options:

```go
type Profile struct {
    Name        string
    Connections int
    Priority    string // aggressive, balanced, polite
}

var Profiles = map[string]Profile{
    "light":  {Name: "light", Connections: 4, Priority: "polite"},
    "medium": {Name: "medium", Connections: 16, Priority: "balanced"},
    "full":   {Name: "full", Connections: 64, Priority: "aggressive"},
}

func GetProfile(name string) (Profile, bool)
```

Tests: lookup known profiles, unknown returns false.

---

### Task 2: Schedule Parser

**Files:**
- Create: `core/scheduler/schedule.go`
- Create: `core/scheduler/schedule_test.go`

```go
type ScheduleEntry struct {
    Days    []time.Weekday
    Start   string // "20:00"
    End     string // "23:00"  
    Profile string
}

type Scheduler struct {
    entries []ScheduleEntry
}

func NewScheduler(entries []ScheduleEntry) *Scheduler
func (s *Scheduler) ActiveProfile(now time.Time) (string, bool)
```

Tests: active during window, inactive outside, handles midnight crossing, multiple entries.

---

### Task 3: Scheduler Loop

**Files:**
- Modify: `core/scheduler/schedule.go`

Add `RunLoop(ctx, engine, onProfileChange)` that checks every 30s if a schedule window is active, starts/stops the engine accordingly.

---

### Task 4: Daemon Service Files

**Files:**
- Create: `core/daemon/daemon.go`
- Create: `core/daemon/daemon_darwin.go`
- Create: `core/daemon/daemon_linux.go`

```go
func Install(execPath string, args []string) error  // writes plist/unit file
func Uninstall() error                               // removes service
func IsInstalled() bool
```

macOS: writes `~/Library/LaunchAgents/com.networkbooster.plist`
Linux: writes `~/.config/systemd/user/networkbooster.service`

---

### Task 5: CLI Commands

**Files:**
- Modify: `cmd/cli/main.go`
- Modify: `config/config.go`

Add commands: `schedule`, `daemon install`, `daemon uninstall`
Add config: `schedule` entries array

---

### Task 6: Integration Test

Build, verify `schedule` and `daemon` commands work.
