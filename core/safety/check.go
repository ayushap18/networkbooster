package safety

// Action represents what the safety system wants the engine to do.
type Action int

const (
	ActionNone     Action = iota
	ActionThrottle        // reduce connections
	ActionPause           // stop all activity
	ActionResume          // resume from pause
)

// CheckResult is the outcome of a single safety check evaluation.
type CheckResult struct {
	Action Action
	Reason string
	Target int // target connection count for ActionThrottle (0 = use default)
}

// Check evaluates a safety condition and returns an action.
type Check interface {
	Name() string
	Evaluate(state State) CheckResult
}

// State holds the current system state for safety evaluation.
type State struct {
	CurrentDownloadMbps float64
	CurrentUploadMbps   float64
	TotalDownloadBytes  int64
	TotalUploadBytes    int64
	ActiveConnections   int
	CPUPercent          float64
	TempCelsius         float64
}
