# Phase 2: TUI Display + History + Rich Metrics — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the basic `\r` line output with a rich bubbletea TUI showing live speed gauges, per-server stats, and connection info. Add SQLite-backed history storage with session summaries and a `history` CLI command.

**Architecture:** Extends Phase 1. New packages: `core/metrics/history` (SQLite persistence), `ui/tui` (bubbletea model/view). Modified: `cmd/cli/main.go` (switch to TUI), `core/metrics/collector.go` (add per-server tracking + peak/avg).

**Tech Stack:** bubbletea + lipgloss + bubbles (TUI), go-sqlite3 or modernc.org/sqlite (pure Go SQLite, no CGO needed for cross-compile)

---

## File Structure (new/modified)

```
networkbooster/
├── core/
│   └── metrics/
│       ├── collector.go          # MODIFY: add per-server stats, peak/avg tracking
│       ├── collector_test.go     # MODIFY: add tests for new metrics
│       ├── history.go            # CREATE: SQLite session/history storage
│       └── history_test.go       # CREATE: history tests
├── ui/
│   └── tui/
│       ├── model.go              # CREATE: bubbletea model (state + update)
│       ├── view.go               # CREATE: bubbletea view (rendering)
│       ├── styles.go             # CREATE: lipgloss styles
│       └── tui.go                # CREATE: public Run() entry point
├── cmd/
│   └── cli/
│       └── main.go               # MODIFY: use TUI, add history command
```

---

### Task 1: Enhanced Metrics Collector

**Files:**
- Modify: `core/metrics/collector.go`
- Modify: `core/metrics/collector_test.go`

- [ ] **Step 1: Write failing tests for per-server and peak/avg metrics**

Add to `core/metrics/collector_test.go`:

```go
func TestCollector_PerServerStats(t *testing.T) {
	c := metrics.NewCollector()

	c.RecordServerBytes("server-1", metrics.DirectionDownload, 1000)
	c.RecordServerBytes("server-1", metrics.DirectionDownload, 500)
	c.RecordServerBytes("server-2", metrics.DirectionDownload, 2000)

	snap := c.Snapshot()
	assert.Equal(t, int64(1500), snap.TotalDownloadBytes)
	assert.Equal(t, int64(2000), snap.TotalDownloadBytes) // total is 3500 actually

	servers := snap.ServerStats
	assert.Len(t, servers, 2)
	assert.Equal(t, int64(1500), servers["server-1"].DownloadBytes)
	assert.Equal(t, int64(2000), servers["server-2"].DownloadBytes)
}

func TestCollector_PeakSpeed(t *testing.T) {
	c := metrics.NewCollector()

	// Record some bytes, take snapshot to get a speed reading
	c.RecordBytes(metrics.DirectionDownload, 5*1024*1024)
	time.Sleep(150 * time.Millisecond)
	snap1 := c.Snapshot()

	// Record more
	c.RecordBytes(metrics.DirectionDownload, 1*1024*1024)
	time.Sleep(150 * time.Millisecond)
	snap2 := c.Snapshot()

	// Peak should be >= the max of the two readings
	assert.GreaterOrEqual(t, snap2.PeakDownloadMbps, snap2.DownloadMbps)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./core/metrics/... -v -run "TestCollector_PerServer|TestCollector_Peak"
```

- [ ] **Step 3: Update collector with per-server stats and peak tracking**

Add to `collector.go`:

```go
type ServerStat struct {
	DownloadBytes int64
	UploadBytes   int64
}

// Updated Snapshot struct - add fields:
// ServerStats       map[string]ServerStat
// PeakDownloadMbps  float64
// PeakUploadMbps    float64
// AvgDownloadMbps   float64
// AvgUploadMbps     float64

// Add to Collector struct:
// serverStats  map[string]*serverAccum  (protected by mu)
// peakDl, peakUl float64
// speedSamples int
// totalDlMbps, totalUlMbps float64  (for running average)
```

Add `RecordServerBytes(serverID string, dir Direction, n int64)` that calls `RecordBytes` AND tracks per-server totals. Update `Snapshot()` to include per-server stats and update peak/avg.

- [ ] **Step 4: Run tests**

```bash
go test ./core/metrics/... -v
```

- [ ] **Step 5: Commit**

```bash
git add core/metrics/
git commit -m "feat: add per-server stats and peak/avg speed tracking to metrics"
```

---

### Task 2: SQLite History Storage

**Files:**
- Create: `core/metrics/history.go`
- Create: `core/metrics/history_test.go`

- [ ] **Step 1: Write failing tests for history**

Create `core/metrics/history_test.go` (add new tests, don't replace existing file):

```go
package metrics_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory_SaveAndListSessions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	h, err := metrics.NewHistory(dbPath)
	require.NoError(t, err)
	defer h.Close()

	session := metrics.Session{
		StartTime:      time.Now().Add(-5 * time.Minute),
		EndTime:        time.Now(),
		Mode:           "download",
		Profile:        "medium",
		Connections:    8,
		TotalDownload:  500 * 1024 * 1024,
		TotalUpload:    0,
		PeakDownload:   150.5,
		PeakUpload:     0,
		AvgDownload:    120.3,
		AvgUpload:      0,
	}

	err = h.SaveSession(session)
	require.NoError(t, err)

	sessions, err := h.ListSessions(10)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "download", sessions[0].Mode)
	assert.Equal(t, int64(500*1024*1024), sessions[0].TotalDownload)
}

func TestHistory_TotalStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	h, err := metrics.NewHistory(dbPath)
	require.NoError(t, err)
	defer h.Close()

	h.SaveSession(metrics.Session{
		StartTime: time.Now(), EndTime: time.Now(),
		TotalDownload: 100, TotalUpload: 50,
	})
	h.SaveSession(metrics.Session{
		StartTime: time.Now(), EndTime: time.Now(),
		TotalDownload: 200, TotalUpload: 75,
	})

	stats, err := h.TotalStats()
	require.NoError(t, err)
	assert.Equal(t, int64(300), stats.TotalDownload)
	assert.Equal(t, int64(125), stats.TotalUpload)
	assert.Equal(t, 2, stats.SessionCount)
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Implement history**

Create `core/metrics/history.go`:

```go
package metrics

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Session struct {
	ID            int64
	StartTime     time.Time
	EndTime       time.Time
	Mode          string
	Profile       string
	Connections   int
	TotalDownload int64
	TotalUpload   int64
	PeakDownload  float64
	PeakUpload    float64
	AvgDownload   float64
	AvgUpload     float64
}

type TotalStats struct {
	TotalDownload int64
	TotalUpload   int64
	SessionCount  int
}

type History struct {
	db *sql.DB
}

func NewHistory(dbPath string) (*History, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		start_time DATETIME,
		end_time DATETIME,
		mode TEXT,
		profile TEXT,
		connections INTEGER,
		total_download INTEGER,
		total_upload INTEGER,
		peak_download REAL,
		peak_upload REAL,
		avg_download REAL,
		avg_upload REAL
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &History{db: db}, nil
}

func (h *History) Close() error { return h.db.Close() }

func (h *History) SaveSession(s Session) error {
	_, err := h.db.Exec(
		`INSERT INTO sessions (start_time, end_time, mode, profile, connections,
		total_download, total_upload, peak_download, peak_upload, avg_download, avg_upload)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.StartTime, s.EndTime, s.Mode, s.Profile, s.Connections,
		s.TotalDownload, s.TotalUpload, s.PeakDownload, s.PeakUpload,
		s.AvgDownload, s.AvgUpload,
	)
	return err
}

func (h *History) ListSessions(limit int) ([]Session, error) {
	rows, err := h.db.Query(
		`SELECT id, start_time, end_time, mode, profile, connections,
		total_download, total_upload, peak_download, peak_upload, avg_download, avg_upload
		FROM sessions ORDER BY start_time DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var s Session
		err := rows.Scan(&s.ID, &s.StartTime, &s.EndTime, &s.Mode, &s.Profile,
			&s.Connections, &s.TotalDownload, &s.TotalUpload, &s.PeakDownload,
			&s.PeakUpload, &s.AvgDownload, &s.AvgUpload)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (h *History) TotalStats() (TotalStats, error) {
	var stats TotalStats
	err := h.db.QueryRow(
		`SELECT COALESCE(SUM(total_download),0), COALESCE(SUM(total_upload),0), COUNT(*)
		FROM sessions`).Scan(&stats.TotalDownload, &stats.TotalUpload, &stats.SessionCount)
	return stats, err
}
```

- [ ] **Step 4: Install sqlite dep and run tests**

```bash
go get modernc.org/sqlite
go mod tidy
go test ./core/metrics/... -v
```

- [ ] **Step 5: Commit**

```bash
git add core/metrics/history.go core/metrics/history_test.go go.mod go.sum
git commit -m "feat: add SQLite session history storage"
```

---

### Task 3: TUI Styles

**Files:**
- Create: `ui/tui/styles.go`

- [ ] **Step 1: Create lipgloss styles**

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF88")).
			MarginBottom(1)

	speedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00BFFF"))

	uploadSpeedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	barFullStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF88"))

	barEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#333333"))

	serverHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFD700")).
			MarginTop(1)

	statusRunning = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF88")).
			Bold(true)

	statusStopped = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			MarginTop(1)
)
```

- [ ] **Step 2: Verify it compiles**

```bash
go get github.com/charmbracelet/lipgloss
go build ./ui/tui/...
```

- [ ] **Step 3: Commit**

```bash
git add ui/tui/styles.go go.mod go.sum
git commit -m "feat: add lipgloss styles for TUI"
```

---

### Task 4: TUI Model + Update

**Files:**
- Create: `ui/tui/model.go`

- [ ] **Step 1: Create bubbletea model**

```go
package tui

import (
	"time"

	"github.com/ayush18/networkbooster/core/engine"
	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

type Model struct {
	engine  *engine.Engine
	compact bool
	quitting bool
	width   int
	height  int
}

func NewModel(eng *engine.Engine, compact bool) Model {
	return Model{
		engine:  eng,
		compact: compact,
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tickCmd()
	}
	return m, nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go get github.com/charmbracelet/bubbletea
go build ./ui/tui/...
```

- [ ] **Step 3: Commit**

```bash
git add ui/tui/model.go go.mod go.sum
git commit -m "feat: add bubbletea TUI model with tick updates"
```

---

### Task 5: TUI View (Rendering)

**Files:**
- Create: `ui/tui/view.go`

- [ ] **Step 1: Create the view rendering**

```go
package tui

import (
	"fmt"
	"strings"
	"time"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	status := m.engine.Status()
	s := status.Snapshot

	if m.compact {
		return m.compactView(s)
	}
	return m.fullView(status)
}

func (m Model) compactView(s /* metrics.Snapshot */) string {
	return fmt.Sprintf("  ↓ %.1f Mbps  ↑ %.1f Mbps  | %d conns | ↓ %.1f MB  ↑ %.1f MB | %s",
		s.DownloadMbps, s.UploadMbps, s.ActiveConnections,
		float64(s.TotalDownloadBytes)/(1024*1024),
		float64(s.TotalUploadBytes)/(1024*1024),
		s.Elapsed.Round(time.Second),
	)
}

func (m Model) fullView(status engine.Status) string {
	s := status.Snapshot
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("⚡ NetworkBooster"))
	b.WriteString("\n\n")

	// Speed gauges
	dlSpeed := fmt.Sprintf("%.1f Mbps", s.DownloadMbps)
	ulSpeed := fmt.Sprintf("%.1f Mbps", s.UploadMbps)

	dlLine := fmt.Sprintf("  %s %s  %s",
		labelStyle.Render("↓ Download:"),
		speedStyle.Render(dlSpeed),
		renderBar(s.DownloadMbps, s.PeakDownloadMbps, 30))
	ulLine := fmt.Sprintf("  %s %s  %s",
		labelStyle.Render("↑ Upload:  "),
		uploadSpeedStyle.Render(ulSpeed),
		renderBar(s.UploadMbps, s.PeakUploadMbps, 30))

	b.WriteString(dlLine + "\n")
	b.WriteString(ulLine + "\n\n")

	// Stats box
	statsContent := fmt.Sprintf(
		"%s %s    %s %s    %s %s\n%s %s    %s %s    %s %s",
		labelStyle.Render("Peak ↓:"), valueStyle.Render(fmt.Sprintf("%.1f Mbps", s.PeakDownloadMbps)),
		labelStyle.Render("Avg ↓:"), valueStyle.Render(fmt.Sprintf("%.1f Mbps", s.AvgDownloadMbps)),
		labelStyle.Render("Total ↓:"), valueStyle.Render(formatBytes(s.TotalDownloadBytes)),
		labelStyle.Render("Peak ↑:"), valueStyle.Render(fmt.Sprintf("%.1f Mbps", s.PeakUploadMbps)),
		labelStyle.Render("Avg ↑:"), valueStyle.Render(fmt.Sprintf("%.1f Mbps", s.AvgUploadMbps)),
		labelStyle.Render("Total ↑:"), valueStyle.Render(formatBytes(s.TotalUploadBytes)),
	)
	b.WriteString(boxStyle.Render(statsContent))
	b.WriteString("\n\n")

	// Connection info
	b.WriteString(fmt.Sprintf("  %s %s    %s %s    %s %s\n",
		labelStyle.Render("Connections:"), valueStyle.Render(fmt.Sprintf("%d", s.ActiveConnections)),
		labelStyle.Render("Elapsed:"), valueStyle.Render(s.Elapsed.Round(time.Second).String()),
		labelStyle.Render("Status:"), statusRunning.Render("● Running"),
	))

	// Per-server table
	if len(s.ServerStats) > 0 {
		b.WriteString("\n")
		b.WriteString(serverHeaderStyle.Render("  Servers"))
		b.WriteString("\n")
		for id, stat := range s.ServerStats {
			b.WriteString(fmt.Sprintf("    %s  ↓ %s  ↑ %s\n",
				labelStyle.Render(id),
				valueStyle.Render(formatBytes(stat.DownloadBytes)),
				valueStyle.Render(formatBytes(stat.UploadBytes)),
			))
		}
	}

	// Help
	b.WriteString(helpStyle.Render("  Press q or Ctrl+C to stop"))

	return b.String()
}

func renderBar(current, max float64, width int) string {
	if max <= 0 {
		max = 1
	}
	ratio := current / max
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	empty := width - filled
	return barFullStyle.Render(strings.Repeat("█", filled)) +
		barEmptyStyle.Render(strings.Repeat("░", empty))
}

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", float64(b)/(1024*1024*1024))
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
```

NOTE: The exact field names in `Snapshot` must match what Task 1 adds (`PeakDownloadMbps`, `PeakUploadMbps`, `AvgDownloadMbps`, `AvgUploadMbps`, `ServerStats map[string]ServerStat`). Adjust if the implementation differs.

- [ ] **Step 2: Verify it compiles**

```bash
go build ./ui/tui/...
```

- [ ] **Step 3: Commit**

```bash
git add ui/tui/view.go
git commit -m "feat: add TUI view with speed gauges, stats, and server table"
```

---

### Task 6: TUI Entry Point

**Files:**
- Create: `ui/tui/tui.go`

- [ ] **Step 1: Create the Run function**

```go
package tui

import (
	"fmt"

	"github.com/ayush18/networkbooster/core/engine"
	tea "github.com/charmbracelet/bubbletea"
)

func Run(eng *engine.Engine, compact bool) error {
	model := NewModel(eng, compact)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./ui/tui/...
```

- [ ] **Step 3: Commit**

```bash
git add ui/tui/tui.go
git commit -m "feat: add TUI entry point with alt-screen support"
```

---

### Task 7: Wire TUI + History into CLI

**Files:**
- Modify: `cmd/cli/main.go`

- [ ] **Step 1: Update CLI to use TUI and add history command**

Major changes to `cmd/cli/main.go`:
- Import `ui/tui` and `core/metrics` (for History)
- Replace the `\r` print loop in `runStart` with `tui.Run(eng, compact)`
- Add `--compact` flag to start command
- On quit, save session to history via `metrics.History`
- Add `historyCmd` that reads from SQLite and prints past sessions
- Add history DB path from config dir (`~/.networkbooster/history.db`)

Key code for `runStart` after engine starts:
```go
// Run TUI (blocks until quit)
err = tui.Run(eng, compactFlag)

// After TUI exits, stop engine and save session
eng.Stop()
status := eng.Status()

// Save to history
histPath := filepath.Join(homeDir, ".networkbooster", "history.db")
hist, err := metrics.NewHistory(histPath)
if err == nil {
    defer hist.Close()
    hist.SaveSession(metrics.Session{...snapshot data...})
}
```

Key code for `historyCmd`:
```go
var historyCmd = &cobra.Command{
    Use: "history",
    Short: "Show session history",
    RunE: func(cmd *cobra.Command, args []string) error {
        hist, err := metrics.NewHistory(histPath)
        // list sessions, print table
    },
}
```

- [ ] **Step 2: Build and verify**

```bash
go mod tidy
make cli
./bin/networkbooster --help
./bin/networkbooster history
```

- [ ] **Step 3: Run all tests**

```bash
go test ./... -v -timeout 30s
```

- [ ] **Step 4: Commit**

```bash
git add cmd/cli/main.go
git commit -m "feat: integrate TUI display and session history into CLI"
```

---

### Task 8: Integration Verification

- [ ] **Step 1: Build and run**

```bash
make cli
./bin/networkbooster start --mode download --connections 4 --compact
# Run for ~5 seconds, press q
```

Expected: Compact single-line TUI with live stats.

- [ ] **Step 2: Run full TUI**

```bash
./bin/networkbooster start --mode download --connections 4
# Run for ~5 seconds, press q
```

Expected: Full TUI with speed gauges, bar charts, stats box, server table.

- [ ] **Step 3: Check history**

```bash
./bin/networkbooster history
```

Expected: Shows the sessions from steps 1 and 2.

- [ ] **Step 4: Run all tests**

```bash
go test ./... -v -timeout 30s
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "Phase 2 complete: TUI display, rich metrics, session history"
```
