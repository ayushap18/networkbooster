package safety_test

import (
	"testing"

	"github.com/ayush18/networkbooster/core/safety"
	"github.com/stretchr/testify/assert"
)

// --- Bandwidth Check ---

func TestBandwidthCheck_NoLimit(t *testing.T) {
	c := safety.NewBandwidthCheck(0, 0)
	result := c.Evaluate(safety.State{CurrentDownloadMbps: 500})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestBandwidthCheck_OverLimit(t *testing.T) {
	c := safety.NewBandwidthCheck(100, 0)
	result := c.Evaluate(safety.State{
		CurrentDownloadMbps: 120,
		ActiveConnections:   8,
	})
	assert.Equal(t, safety.ActionThrottle, result.Action)
	assert.Less(t, result.Target, 8)
	assert.Greater(t, result.Target, 0)
}

func TestBandwidthCheck_UnderLimit(t *testing.T) {
	c := safety.NewBandwidthCheck(100, 0)
	result := c.Evaluate(safety.State{CurrentDownloadMbps: 80, ActiveConnections: 8})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestBandwidthCheck_UploadOverLimit(t *testing.T) {
	c := safety.NewBandwidthCheck(0, 50)
	result := c.Evaluate(safety.State{
		CurrentUploadMbps: 70,
		ActiveConnections: 8,
	})
	assert.Equal(t, safety.ActionThrottle, result.Action)
}

// --- Data Limit Check ---

func TestDataLimitCheck_UnderLimit(t *testing.T) {
	c := safety.NewDataLimitCheck(10 * 1024 * 1024 * 1024)
	result := c.Evaluate(safety.State{TotalDownloadBytes: 5 * 1024 * 1024 * 1024})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestDataLimitCheck_AtWarning(t *testing.T) {
	limit := int64(10 * 1024 * 1024 * 1024) // 10 GB
	c := safety.NewDataLimitCheck(limit)
	// 8.5 GB = 85% -> should throttle
	result := c.Evaluate(safety.State{TotalDownloadBytes: int64(float64(limit) * 0.85)})
	assert.Equal(t, safety.ActionThrottle, result.Action)
}

func TestDataLimitCheck_AtLimit(t *testing.T) {
	limit := int64(10 * 1024 * 1024 * 1024)
	c := safety.NewDataLimitCheck(limit)
	result := c.Evaluate(safety.State{
		TotalDownloadBytes: 9 * 1024 * 1024 * 1024,
		TotalUploadBytes:   2 * 1024 * 1024 * 1024,
	})
	assert.Equal(t, safety.ActionPause, result.Action)
}

func TestDataLimitCheck_NoLimit(t *testing.T) {
	c := safety.NewDataLimitCheck(0)
	result := c.Evaluate(safety.State{TotalDownloadBytes: 999 * 1024 * 1024 * 1024})
	assert.Equal(t, safety.ActionNone, result.Action)
}

// --- CPU Check ---

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
	assert.Greater(t, result.Target, 0)
}

func TestCPUCheck_NoLimit(t *testing.T) {
	c := safety.NewCPUCheck(0)
	result := c.Evaluate(safety.State{CPUPercent: 99})
	assert.Equal(t, safety.ActionNone, result.Action)
}

// --- Temperature Check ---

func TestTempCheck_Normal(t *testing.T) {
	c := safety.NewTemperatureCheck(85)
	result := c.Evaluate(safety.State{TempCelsius: 65})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestTempCheck_Warning(t *testing.T) {
	c := safety.NewTemperatureCheck(85)
	// Warning starts at 80 (85-5), this is 82 -> throttle
	result := c.Evaluate(safety.State{TempCelsius: 82, ActiveConnections: 8})
	assert.Equal(t, safety.ActionThrottle, result.Action)
	assert.Greater(t, result.Target, 0)
}

func TestTempCheck_Critical(t *testing.T) {
	c := safety.NewTemperatureCheck(85)
	// Critical at 90 (85+5)
	result := c.Evaluate(safety.State{TempCelsius: 92})
	assert.Equal(t, safety.ActionPause, result.Action)
}

func TestTempCheck_NoSensor(t *testing.T) {
	c := safety.NewTemperatureCheck(85)
	result := c.Evaluate(safety.State{TempCelsius: 0})
	assert.Equal(t, safety.ActionNone, result.Action)
}

// --- Monitor ---

type fakeCheck struct {
	name   string
	result safety.CheckResult
}

func (f *fakeCheck) Name() string                      { return f.name }
func (f *fakeCheck) Evaluate(s safety.State) safety.CheckResult { return f.result }

func TestMonitor_RunsChecks(t *testing.T) {
	alwaysThrottle := &fakeCheck{
		name:   "test",
		result: safety.CheckResult{Action: safety.ActionThrottle, Target: 2, Reason: "test"},
	}

	m := safety.NewMonitor([]safety.Check{alwaysThrottle})
	result := m.Evaluate(safety.State{ActiveConnections: 8})
	assert.Equal(t, safety.ActionThrottle, result.Action)
	assert.Equal(t, 2, result.Target)
}

func TestMonitor_MostSevereWins(t *testing.T) {
	throttle := &fakeCheck{
		name:   "throttle",
		result: safety.CheckResult{Action: safety.ActionThrottle, Target: 4},
	}
	pause := &fakeCheck{
		name:   "pause",
		result: safety.CheckResult{Action: safety.ActionPause, Reason: "critical"},
	}

	m := safety.NewMonitor([]safety.Check{throttle, pause})
	result := m.Evaluate(safety.State{})
	assert.Equal(t, safety.ActionPause, result.Action)
}

func TestMonitor_AllNone(t *testing.T) {
	ok1 := &fakeCheck{name: "ok1", result: safety.CheckResult{Action: safety.ActionNone}}
	ok2 := &fakeCheck{name: "ok2", result: safety.CheckResult{Action: safety.ActionNone}}

	m := safety.NewMonitor([]safety.Check{ok1, ok2})
	result := m.Evaluate(safety.State{})
	assert.Equal(t, safety.ActionNone, result.Action)
}

func TestMonitor_LowestThrottleTarget(t *testing.T) {
	t1 := &fakeCheck{
		name:   "t1",
		result: safety.CheckResult{Action: safety.ActionThrottle, Target: 6},
	}
	t2 := &fakeCheck{
		name:   "t2",
		result: safety.CheckResult{Action: safety.ActionThrottle, Target: 3},
	}

	m := safety.NewMonitor([]safety.Check{t1, t2})
	result := m.Evaluate(safety.State{})
	assert.Equal(t, safety.ActionThrottle, result.Action)
	assert.Equal(t, 3, result.Target, "should pick the most conservative target")
}
