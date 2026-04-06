package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockConnSetter struct {
	conns int
}

func (m *mockConnSetter) SetConnections(n int) { m.conns = n }
func (m *mockConnSetter) ConnectionCount() int { return m.conns }
func (m *mockConnSetter) IsPaused() bool       { return false }

// TestScaler_RampUp: 3 increasing samples (~17% increase) should cause connections to increase.
func TestScaler_RampUp(t *testing.T) {
	mock := &mockConnSetter{conns: 4}
	s := NewScaler(ScalerOptions{MinConnections: 2, MaxConnections: 64}, mock)

	initial := mock.conns
	s.RecordSample(100)
	s.RecordSample(108)
	s.RecordSample(117) // ~17% increase from 100

	assert.Greater(t, mock.conns, initial, "connections should have increased on strong throughput improvement")
}

// TestScaler_Plateau_ThenBackoff: 9 flat samples (3 stall cycles) should decrease connections.
func TestScaler_Plateau_ThenBackoff(t *testing.T) {
	mock := &mockConnSetter{conns: 10}
	s := NewScaler(ScalerOptions{MinConnections: 2, MaxConnections: 64}, mock)

	initial := mock.conns

	// Feed 9 flat samples across 3 stall cycles.
	// Each group of 3 has delta < 0.01; after 3 stall counts the scaler removes a connection.
	for i := 0; i < 9; i++ {
		s.RecordSample(100) // completely flat, delta == 0
	}

	assert.Less(t, mock.conns, initial, "connections should have decreased after repeated stall cycles")
}

// TestScaler_RespectsMax: starting near max, increasing samples should never exceed MaxConnections.
func TestScaler_RespectsMax(t *testing.T) {
	mock := &mockConnSetter{conns: 19}
	s := NewScaler(ScalerOptions{MinConnections: 2, MaxConnections: 20}, mock)

	// Feed strongly improving samples multiple times.
	for i := 0; i < 6; i++ {
		s.RecordSample(float64(100 + i*20)) // large ramp-up
	}

	assert.LessOrEqual(t, mock.conns, 20, "connections must not exceed MaxConnections")
}

// TestScaler_RespectsMin: starting just above min, flat samples should never go below MinConnections.
func TestScaler_RespectsMin(t *testing.T) {
	mock := &mockConnSetter{conns: 3}
	s := NewScaler(ScalerOptions{MinConnections: 2, MaxConnections: 64}, mock)

	// Feed many flat samples to trigger multiple backoff cycles.
	for i := 0; i < 30; i++ {
		s.RecordSample(100)
	}

	assert.GreaterOrEqual(t, mock.conns, 2, "connections must not go below MinConnections")
}

// TestScaler_ResetHistory: after feeding 2 samples and resetting, a single new sample should not trigger scaling.
func TestScaler_ResetHistory(t *testing.T) {
	mock := &mockConnSetter{conns: 8}
	s := NewScaler(ScalerOptions{MinConnections: 2, MaxConnections: 64}, mock)

	s.RecordSample(100)
	s.RecordSample(200) // only 2 samples — no action yet
	s.ResetHistory()

	before := mock.conns
	s.RecordSample(300) // only 1 sample after reset — still no action

	assert.Equal(t, before, mock.conns, "no scaling action should occur with fewer than 3 history samples after reset")
}
