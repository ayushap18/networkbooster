package safety

// DataLimitCheck pauses when total data transferred exceeds the limit.
// Warns (throttles) at 80% of limit, pauses at 100%.
type DataLimitCheck struct {
	limitBytes int64
}

func NewDataLimitCheck(limitBytes int64) *DataLimitCheck {
	return &DataLimitCheck{limitBytes: limitBytes}
}

func (d *DataLimitCheck) Name() string { return "data-limit" }

func (d *DataLimitCheck) Evaluate(s State) CheckResult {
	if d.limitBytes <= 0 {
		return CheckResult{Action: ActionNone}
	}

	totalBytes := s.TotalDownloadBytes + s.TotalUploadBytes
	ratio := float64(totalBytes) / float64(d.limitBytes)

	if ratio >= 1.0 {
		return CheckResult{Action: ActionPause, Reason: "data limit reached"}
	}
	if ratio >= 0.8 {
		// Throttle to half connections when approaching limit
		target := s.ActiveConnections / 2
		if target < 1 {
			target = 1
		}
		return CheckResult{Action: ActionThrottle, Reason: "approaching data limit (80%)", Target: target}
	}
	return CheckResult{Action: ActionNone}
}
