package safety

// CPUCheck throttles when CPU usage exceeds the threshold.
type CPUCheck struct {
	maxPercent float64
}

func NewCPUCheck(maxPercent float64) *CPUCheck {
	return &CPUCheck{maxPercent: maxPercent}
}

func (c *CPUCheck) Name() string { return "cpu" }

func (c *CPUCheck) Evaluate(s State) CheckResult {
	if c.maxPercent <= 0 {
		return CheckResult{Action: ActionNone}
	}
	if s.CPUPercent > c.maxPercent {
		// Reduce connections proportionally
		ratio := c.maxPercent / s.CPUPercent
		target := int(float64(s.ActiveConnections) * ratio)
		if target < 1 {
			target = 1
		}
		return CheckResult{Action: ActionThrottle, Reason: "CPU usage too high", Target: target}
	}
	return CheckResult{Action: ActionNone}
}
