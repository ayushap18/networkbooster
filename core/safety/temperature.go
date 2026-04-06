package safety

// TemperatureCheck throttles at warning temp and pauses at critical temp.
// Warning = threshold - 5, Critical = threshold + 5.
type TemperatureCheck struct {
	maxCelsius float64
}

func NewTemperatureCheck(maxCelsius float64) *TemperatureCheck {
	return &TemperatureCheck{maxCelsius: maxCelsius}
}

func (t *TemperatureCheck) Name() string { return "temperature" }

func (t *TemperatureCheck) Evaluate(s State) CheckResult {
	if t.maxCelsius <= 0 || s.TempCelsius <= 0 {
		return CheckResult{Action: ActionNone}
	}

	critical := t.maxCelsius + 5
	warning := t.maxCelsius - 5

	if s.TempCelsius >= critical {
		return CheckResult{Action: ActionPause, Reason: "temperature critical"}
	}
	if s.TempCelsius >= warning {
		// Reduce connections by proportion of how close to critical
		ratio := (critical - s.TempCelsius) / (critical - warning)
		target := int(float64(s.ActiveConnections) * ratio)
		if target < 1 {
			target = 1
		}
		return CheckResult{Action: ActionThrottle, Reason: "temperature warning", Target: target}
	}
	return CheckResult{Action: ActionNone}
}
