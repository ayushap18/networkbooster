# Phase 3: Safety System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add safety controls that throttle or pause the engine: bandwidth caps, data usage limits, CPU monitoring, temperature monitoring, and app-aware network priority. Safety checks run every 500ms and can reduce connections or pause the engine.

**Architecture:** New package `core/safety` with a `Monitor` that wraps the engine. Each safety check is a `Check` interface implementation. The monitor runs a loop, evaluates all checks, and issues throttle/pause commands to the engine. Config extended with safety fields.

**Tech Stack:** Go stdlib for CPU/temp (os-specific syscalls), gopsutil for cross-platform system metrics as fallback.

---

## File Structure

```
networkbooster/
├── core/
│   ├── engine/
│   │   └── engine.go              # MODIFY: add SetConnections(), Pause(), Resume()
│   └── safety/
│       ├��─ check.go               # Check interface + Action enum
│       ├── bandwidth.go           # Bandwidth cap check
│       ├── bandwidth_test.go
│       ├── datalimit.go           # Data usage limit check
│       ├── datalimit_test.go
│       ├── cpu.go                 # CPU usage check
│       ├─��� cpu_test.go
│       ├── temperature.go         # Temperature check (platform-specific)
│       ├── temperature_test.go
│       ├── monitor.go             # Monitor loop that runs all checks
│       └── monitor_test.go
├── config/
│   └── config.go                  # MODIFY: add Safety config fields
```

---

### Task 1: Engine Throttle API

**Files:**
- Modify: `core/engine/engine.go`
- Modify: `core/engine/engine_test.go`

Add methods to the engine that safety can call:
- `SetConnections(n int)` — dynamically adjust active worker count
- `Pause()` — stop all workers temporarily
- `Resume()` — restart workers after pause
- `IsPaused() bool`

- [ ] **Step 1: Write failing tests**

Add to `core/engine/engine_test.go`:

```go
func TestEngine_Pause_Resume(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{Connections: 2})
	eng.Start(engine.ModeDownload)
	defer eng.Stop()

	time.Sleep(200 * time.Millisecond)
	assert.Greater(t, eng.Status().Snapshot.ActiveConnections, 0)

	eng.Pause()
	time.Sleep(200 * time.Millisecond)
	assert.True(t, eng.IsPaused())
	assert.Equal(t, 0, eng.Status().Snapshot.ActiveConnections)

	eng.Resume()
	time.Sleep(200 * time.Millisecond)
	assert.False(t, eng.IsPaused())
	assert.Greater(t, eng.Status().Snapshot.ActiveConnections, 0)
}
```

- [ ] **Step 2: Implement Pause/Resume**

The engine needs to track its `context.CancelFunc` and be able to cancel all workers then restart them. Add:

```go
func (e *Engine) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running || e.paused { return }
	e.cancel()
	e.mu.Unlock()
	e.wg.Wait()
	e.mu.Lock()
	e.paused = true
}

func (e *Engine) Resume() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running || !e.paused { return }
	e.paused = false
	// Re-discover and re-launch workers with same mode
	// ... (reuse Start logic but without resetting collector)
}

func (e *Engine) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}
```

- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

---

### Task 2: Safety Check Interface

**Files:**
- Create: `core/safety/check.go`

- [ ] **Step 1: Define the Check interface and Action types**

```go
package safety

type Action int

const (
	ActionNone Action = iota
	ActionThrottle  // reduce connections
	ActionPause     // stop all activity
	ActionResume    // resume from pause
)

type CheckResult struct {
	Action  Action
	Reason  string
	Target  int  // target connection count for ActionThrottle (0 = use default)
}

type Check interface {
	Name() string
	Evaluate(state State) CheckResult
}

type State struct {
	CurrentDownloadMbps float64
	CurrentUploadMbps   float64
	TotalDownloadBytes  int64
	TotalUploadBytes    int64
	ActiveConnections   int
	CPUPercent          float64
	TempCelsius         float64
}
```

- [ ] **Step 2: Verify it compiles**
- [ ] **Step 3: Commit**

---

### Task 3: Bandwidth Cap Check

**Files:**
- Create: `core/safety/bandwidth.go`
- Create: `core/safety/bandwidth_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestBandwidthCheck_NoLimit(t *testing.T) {
	c := safety.NewBandwidthCheck(0, 0)
	result := c.Evaluate(safety.State{CurrentDownloadMbps: 500})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestBandwidthCheck_OverLimit(t *testing.T) {
	c := safety.NewBandwidthCheck(100, 0) // 100 Mbps download cap
	result := c.Evaluate(safety.State{
		CurrentDownloadMbps: 120,
		ActiveConnections: 8,
	})
	assert.Equal(t, safety.ActionThrottle, result.Action)
	assert.Less(t, result.Target, 8)
}

func TestBandwidthCheck_UnderLimit(t *testing.T) {
	c := safety.NewBandwidthCheck(100, 0)
	result := c.Evaluate(safety.State{CurrentDownloadMbps: 80, ActiveConnections: 8})
	assert.Equal(t, safety.ActionNone, result.Action)
}
```

- [ ] **Step 2: Implement**

```go
type BandwidthCheck struct {
	maxDownloadMbps float64
	maxUploadMbps   float64
}

func NewBandwidthCheck(maxDl, maxUl float64) *BandwidthCheck {
	return &BandwidthCheck{maxDownloadMbps: maxDl, maxUploadMbps: maxUl}
}

func (b *BandwidthCheck) Name() string { return "bandwidth-cap" }

func (b *BandwidthCheck) Evaluate(s State) CheckResult {
	if b.maxDownloadMbps > 0 && s.CurrentDownloadMbps > b.maxDownloadMbps {
		ratio := b.maxDownloadMbps / s.CurrentDownloadMbps
		target := int(float64(s.ActiveConnections) * ratio)
		if target < 1 { target = 1 }
		return CheckResult{Action: ActionThrottle, Reason: "download exceeds cap", Target: target}
	}
	if b.maxUploadMbps > 0 && s.CurrentUploadMbps > b.maxUploadMbps {
		ratio := b.maxUploadMbps / s.CurrentUploadMbps
		target := int(float64(s.ActiveConnections) * ratio)
		if target < 1 { target = 1 }
		return CheckResult{Action: ActionThrottle, Reason: "upload exceeds cap", Target: target}
	}
	return CheckResult{Action: ActionNone}
}
```

- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

---

### Task 4: Data Limit Check

**Files:**
- Create: `core/safety/datalimit.go`
- Create: `core/safety/datalimit_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestDataLimitCheck_UnderLimit(t *testing.T) {
	c := safety.NewDataLimitCheck(10 * 1024 * 1024 * 1024) // 10 GB
	result := c.Evaluate(safety.State{TotalDownloadBytes: 5 * 1024 * 1024 * 1024})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestDataLimitCheck_AtWarning(t *testing.T) {
	c := safety.NewDataLimitCheck(10 * 1024 * 1024 * 1024)
	result := c.Evaluate(safety.State{TotalDownloadBytes: 8.5 * 1024 * 1024 * 1024})
	assert.Equal(t, safety.ActionThrottle, result.Action)
}

func TestDataLimitCheck_AtLimit(t *testing.T) {
	c := safety.NewDataLimitCheck(10 * 1024 * 1024 * 1024)
	result := c.Evaluate(safety.State{
		TotalDownloadBytes: 10 * 1024 * 1024 * 1024,
		TotalUploadBytes: 1 * 1024 * 1024 * 1024,
	})
	assert.Equal(t, safety.ActionPause, result.Action)
}
```

- [ ] **Step 2: Implement** — warn at 80%, pause at 100% of limit, counting both download+upload
- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

---

### Task 5: CPU Check

**Files:**
- Create: `core/safety/cpu.go`
- Create: `core/safety/cpu_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCPUCheck_UnderThreshold(t *testing.T) {
	c := safety.NewCPUCheck(80)
	result := c.Evaluate(safety.State{CPUPercent: 60, ActiveConnections: 8})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestCPUCheck_OverThreshold(t *testing.T) {
	c := safety.NewCPUCheck(80)
	result := c.Evaluate(safety.State{CPUPercent: 90, ActiveConnections: 8})
	assert.Equal(t, safety.ActionThrottle, result.Action)
	assert.Less(t, result.Target, 8)
}
```

- [ ] **Step 2: Implement** — reduce connections proportionally when CPU exceeds threshold
- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

---

### Task 6: Temperature Check

**Files:**
- Create: `core/safety/temperature.go`
- Create: `core/safety/temperature_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestTempCheck_Normal(t *testing.T) {
	c := safety.NewTemperatureCheck(85)
	result := c.Evaluate(safety.State{TempCelsius: 65})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestTempCheck_Warning(t *testing.T) {
	c := safety.NewTemperatureCheck(85)
	result := c.Evaluate(safety.State{TempCelsius: 80, ActiveConnections: 8})
	assert.Equal(t, safety.ActionThrottle, result.Action)
}

func TestTempCheck_Critical(t *testing.T) {
	c := safety.NewTemperatureCheck(85)
	result := c.Evaluate(safety.State{TempCelsius: 90})
	assert.Equal(t, safety.ActionPause, result.Action)
}
```

- [ ] **Step 2: Implement** — throttle at warning (threshold-5), pause at critical (threshold+5)
- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

---

### Task 7: Safety Monitor

**Files:**
- Create: `core/safety/monitor.go`
- Create: `core/safety/monitor_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestMonitor_RunsChecks(t *testing.T) {
	// Create a monitor with a fake check that always returns throttle
	alwaysThrottle := &fakeCheck{result: safety.CheckResult{
		Action: safety.ActionThrottle, Target: 2, Reason: "test",
	}}
	
	m := safety.NewMonitor([]safety.Check{alwaysThrottle})
	result := m.Evaluate(safety.State{ActiveConnections: 8})
	assert.Equal(t, safety.ActionThrottle, result.Action)
}

func TestMonitor_MostSevereWins(t *testing.T) {
	throttle := &fakeCheck{result: safety.CheckResult{Action: safety.ActionThrottle, Target: 4}}
	pause := &fakeCheck{result: safety.CheckResult{Action: safety.ActionPause, Reason: "critical"}}
	
	m := safety.NewMonitor([]safety.Check{throttle, pause})
	result := m.Evaluate(safety.State{})
	assert.Equal(t, safety.ActionPause, result.Action)
}
```

- [ ] **Step 2: Implement** — the monitor evaluates all checks and returns the most severe action
- [ ] **Step 3: Run tests**
- [ ] **Step 4: Commit**

---

### Task 8: Config Safety Fields + CLI Integration

**Files:**
- Modify: `config/config.go`
- Modify: `cmd/cli/main.go`

- [ ] **Step 1: Add safety fields to config**

```go
type SafetyConfig struct {
	MaxDownloadMbps  float64 `yaml:"max_download_mbps"`
	MaxUploadMbps    float64 `yaml:"max_upload_mbps"`
	DailyDataLimitGB float64 `yaml:"daily_data_limit_gb"`
	MaxCPUPercent    float64 `yaml:"max_cpu_percent"`
	MaxTempCelsius   float64 `yaml:"max_temp_celsius"`
	MaxConnections   int     `yaml:"max_connections"`
}

type Config struct {
	// ... existing fields ...
	Safety SafetyConfig `yaml:"safety"`
}
```

- [ ] **Step 2: Wire safety monitor into CLI start command**

In `runStart`, after engine starts:
```go
// Build safety checks from config
var checks []safety.Check
if cfg.Safety.MaxDownloadMbps > 0 || cfg.Safety.MaxUploadMbps > 0 {
	checks = append(checks, safety.NewBandwidthCheck(cfg.Safety.MaxDownloadMbps, cfg.Safety.MaxUploadMbps))
}
// ... etc for each check

monitor := safety.NewMonitor(checks)
// Start monitor loop in background goroutine
```

- [ ] **Step 3: Run all tests**
- [ ] **Step 4: Commit**

---

### Task 9: Integration Verification

- [ ] **Step 1: Build and run all tests**
- [ ] **Step 2: Test with bandwidth cap**
```bash
# Create a config with safety limits
cat > /tmp/nb-test-config.yaml << 'EOF'
mode: download
connections: 8
safety:
  max_download_mbps: 50
  max_cpu_percent: 90
EOF
NETWORKBOOSTER_CONFIG=/tmp/nb-test-config.yaml ./bin/networkbooster start
```
- [ ] **Step 3: Verify throttling behavior in TUI output**
- [ ] **Step 4: Final commit**
