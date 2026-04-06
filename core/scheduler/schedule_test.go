package scheduler_test

import (
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/scheduler"
	"github.com/stretchr/testify/assert"
)

func makeTime(weekday time.Weekday, hour, min int) time.Time {
	// Create a time on a specific weekday
	// Use a known date: 2026-04-06 is a Monday
	base := time.Date(2026, 4, 6, hour, min, 0, 0, time.Local) // Monday
	daysToAdd := int(weekday) - int(time.Monday)
	if daysToAdd < 0 {
		daysToAdd += 7
	}
	return base.AddDate(0, 0, daysToAdd)
}

func TestScheduler_ActiveDuringWindow(t *testing.T) {
	s := scheduler.NewScheduler([]scheduler.ScheduleEntry{
		{
			Days:    []time.Weekday{time.Monday},
			Start:   "20:00",
			End:     "23:00",
			Profile: "full",
		},
	})

	now := makeTime(time.Monday, 21, 30) // Monday 21:30
	profile, ok := s.ActiveProfile(now)
	assert.True(t, ok)
	assert.Equal(t, "full", profile)
}

func TestScheduler_InactiveOutsideWindow(t *testing.T) {
	s := scheduler.NewScheduler([]scheduler.ScheduleEntry{
		{
			Days:    []time.Weekday{time.Monday},
			Start:   "20:00",
			End:     "23:00",
			Profile: "full",
		},
	})

	now := makeTime(time.Monday, 15, 0) // Monday 15:00
	_, ok := s.ActiveProfile(now)
	assert.False(t, ok)
}

func TestScheduler_InactiveWrongDay(t *testing.T) {
	s := scheduler.NewScheduler([]scheduler.ScheduleEntry{
		{
			Days:    []time.Weekday{time.Monday},
			Start:   "20:00",
			End:     "23:00",
			Profile: "full",
		},
	})

	now := makeTime(time.Tuesday, 21, 30) // Tuesday 21:30
	_, ok := s.ActiveProfile(now)
	assert.False(t, ok)
}

func TestScheduler_MidnightCrossing(t *testing.T) {
	s := scheduler.NewScheduler([]scheduler.ScheduleEntry{
		{
			Days:    []time.Weekday{time.Saturday},
			Start:   "22:00",
			End:     "02:00",
			Profile: "medium",
		},
	})

	// Saturday 23:00 should be active
	now := makeTime(time.Saturday, 23, 0)
	profile, ok := s.ActiveProfile(now)
	assert.True(t, ok)
	assert.Equal(t, "medium", profile)

	// Saturday 01:00 should also be active (after midnight)
	now = makeTime(time.Saturday, 1, 0)
	profile, ok = s.ActiveProfile(now)
	assert.True(t, ok)
	assert.Equal(t, "medium", profile)
}

func TestScheduler_MultipleEntries(t *testing.T) {
	s := scheduler.NewScheduler([]scheduler.ScheduleEntry{
		{
			Days:    []time.Weekday{time.Monday, time.Tuesday},
			Start:   "20:00",
			End:     "23:00",
			Profile: "full",
		},
		{
			Days:    []time.Weekday{time.Saturday, time.Sunday},
			Start:   "00:00",
			End:     "06:00",
			Profile: "medium",
		},
	})

	// Tuesday 21:00
	profile, ok := s.ActiveProfile(makeTime(time.Tuesday, 21, 0))
	assert.True(t, ok)
	assert.Equal(t, "full", profile)

	// Sunday 03:00
	profile, ok = s.ActiveProfile(makeTime(time.Sunday, 3, 0))
	assert.True(t, ok)
	assert.Equal(t, "medium", profile)
}

func TestParseDays(t *testing.T) {
	days := scheduler.ParseDays([]string{"mon", "tue", "wed", "thu", "fri"})
	assert.Len(t, days, 5)
	assert.Equal(t, time.Monday, days[0])
	assert.Equal(t, time.Friday, days[4])
}

func TestScheduler_Empty(t *testing.T) {
	s := scheduler.NewScheduler(nil)
	_, ok := s.ActiveProfile(time.Now())
	assert.False(t, ok)
}
