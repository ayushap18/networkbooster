package safety

import (
	"context"
	"log"
	"time"

	"github.com/ayush18/networkbooster/core/metrics"
)

// Monitor runs safety checks periodically and returns actions.
type Monitor struct {
	checks   []Check
	interval time.Duration
}

func NewMonitor(checks []Check) *Monitor {
	return &Monitor{
		checks:   checks,
		interval: 500 * time.Millisecond,
	}
}

// Evaluate runs all checks and returns the most severe result.
// Severity: Pause > Throttle > None
func (m *Monitor) Evaluate(s State) CheckResult {
	var worst CheckResult
	for _, check := range m.checks {
		result := check.Evaluate(s)
		if result.Action > worst.Action {
			worst = result
		} else if result.Action == worst.Action && result.Action == ActionThrottle {
			// For throttle, pick the lower target (more conservative)
			if result.Target < worst.Target {
				worst = result
			}
		}
	}
	return worst
}

// EngineController is the interface the monitor uses to control the engine.
type EngineController interface {
	Status() EngineStatus
	Pause()
	Resume()
	IsPaused() bool
}

// EngineStatus is a simplified status for the monitor.
type EngineStatus struct {
	Snapshot    metrics.Snapshot
	Running     bool
	Paused      bool
	Connections int
}

// SystemStats provides CPU and temperature readings.
type SystemStats interface {
	CPUPercent() float64
	TempCelsius() float64
}

// defaultSystemStats returns zero values (no sensor available).
type defaultSystemStats struct{}

func (d defaultSystemStats) CPUPercent() float64  { return 0 }
func (d defaultSystemStats) TempCelsius() float64 { return 0 }

// RunLoop starts the safety monitor loop. It evaluates checks every 500ms
// and logs actions. Call cancel on the context to stop.
func (m *Monitor) RunLoop(ctx context.Context, collector *metrics.Collector, onAction func(CheckResult)) {
	if len(m.checks) == 0 {
		return
	}

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := collector.Snapshot()
			state := State{
				CurrentDownloadMbps: snap.DownloadMbps,
				CurrentUploadMbps:   snap.UploadMbps,
				TotalDownloadBytes:  snap.TotalDownloadBytes,
				TotalUploadBytes:    snap.TotalUploadBytes,
				ActiveConnections:   snap.ActiveConnections,
			}

			result := m.Evaluate(state)
			if result.Action != ActionNone {
				log.Printf("[safety] %s: %s (action=%d, target=%d)",
					result.Reason, result.Reason, result.Action, result.Target)
				if onAction != nil {
					onAction(result)
				}
			}
		}
	}
}
