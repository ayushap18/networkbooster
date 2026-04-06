package engine

// Profile defines a preset configuration for the engine.
type Profile struct {
	Name        string
	Connections int
	Priority    string // aggressive, balanced, polite
}

// Profiles contains the built-in profile presets.
var Profiles = map[string]Profile{
	"light":  {Name: "light", Connections: 4, Priority: "polite"},
	"medium": {Name: "medium", Connections: 16, Priority: "balanced"},
	"full":   {Name: "full", Connections: 64, Priority: "aggressive"},
}

// GetProfile returns the named profile. Returns false if not found.
func GetProfile(name string) (Profile, bool) {
	p, ok := Profiles[name]
	return p, ok
}
