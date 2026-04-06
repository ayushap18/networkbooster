package engine_test

import (
	"testing"

	"github.com/ayush18/networkbooster/core/engine"
	"github.com/stretchr/testify/assert"
)

func TestGetProfile_Known(t *testing.T) {
	p, ok := engine.GetProfile("light")
	assert.True(t, ok)
	assert.Equal(t, 4, p.Connections)
	assert.Equal(t, "polite", p.Priority)

	p, ok = engine.GetProfile("medium")
	assert.True(t, ok)
	assert.Equal(t, 16, p.Connections)

	p, ok = engine.GetProfile("full")
	assert.True(t, ok)
	assert.Equal(t, 64, p.Connections)
}

func TestGetProfile_Unknown(t *testing.T) {
	_, ok := engine.GetProfile("nonexistent")
	assert.False(t, ok)
}
