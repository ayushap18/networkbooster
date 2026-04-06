package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ScheduleEntry defines a time window with an associated profile.
type ScheduleEntry struct {
	Days    []time.Weekday `yaml:"days"`
	Start   string         `yaml:"start"` // "20:00"
	End     string         `yaml:"end"`   // "23:00"
	Profile string         `yaml:"profile"`
}

// Scheduler evaluates schedule entries to determine which profile is active.
type Scheduler struct {
	entries []ScheduleEntry
}

func NewScheduler(entries []ScheduleEntry) *Scheduler {
	return &Scheduler{entries: entries}
}

// ActiveProfile returns the profile name if a schedule window is currently active.
func (s *Scheduler) ActiveProfile(now time.Time) (string, bool) {
	for _, entry := range s.entries {
		if s.isActive(entry, now) {
			return entry.Profile, true
		}
	}
	return "", false
}

func (s *Scheduler) isActive(e ScheduleEntry, now time.Time) bool {
	// Check day
	dayMatch := false
	for _, d := range e.Days {
		if d == now.Weekday() {
			dayMatch = true
			break
		}
	}
	if !dayMatch {
		return false
	}

	// Parse start/end times
	startH, startM, err := parseTime(e.Start)
	if err != nil {
		return false
	}
	endH, endM, err := parseTime(e.End)
	if err != nil {
		return false
	}

	nowMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM

	// Handle midnight crossing (e.g., 22:00 - 02:00)
	if endMinutes <= startMinutes {
		return nowMinutes >= startMinutes || nowMinutes < endMinutes
	}
	return nowMinutes >= startMinutes && nowMinutes < endMinutes
}

func parseTime(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", s)
	}
	var h, m int
	_, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil {
		return 0, 0, err
	}
	return h, m, nil
}

// ParseDays converts day name strings to time.Weekday values.
func ParseDays(days []string) []time.Weekday {
	dayMap := map[string]time.Weekday{
		"sun": time.Sunday, "sunday": time.Sunday,
		"mon": time.Monday, "monday": time.Monday,
		"tue": time.Tuesday, "tuesday": time.Tuesday,
		"wed": time.Wednesday, "wednesday": time.Wednesday,
		"thu": time.Thursday, "thursday": time.Thursday,
		"fri": time.Friday, "friday": time.Friday,
		"sat": time.Saturday, "saturday": time.Saturday,
	}
	var result []time.Weekday
	for _, d := range days {
		if wd, ok := dayMap[strings.ToLower(d)]; ok {
			result = append(result, wd)
		}
	}
	return result
}

// RunLoop checks the schedule every 30 seconds and calls onStart/onStop
// when entering/leaving a scheduled window.
func (s *Scheduler) RunLoop(ctx context.Context, onStart func(profile string), onStop func()) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var active bool
	var currentProfile string

	// Check immediately on start
	if profile, ok := s.ActiveProfile(time.Now()); ok {
		active = true
		currentProfile = profile
		onStart(profile)
	}

	for {
		select {
		case <-ctx.Done():
			if active {
				onStop()
			}
			return
		case <-ticker.C:
			profile, ok := s.ActiveProfile(time.Now())
			if ok && !active {
				active = true
				currentProfile = profile
				onStart(profile)
			} else if ok && active && profile != currentProfile {
				onStop()
				currentProfile = profile
				onStart(profile)
			} else if !ok && active {
				active = false
				currentProfile = ""
				onStop()
			}
		}
	}
}
