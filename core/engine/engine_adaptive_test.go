package engine_test

import (
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_ConnectionCount(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{Connections: 4})
	err := eng.Start(engine.ModeDownload)
	require.NoError(t, err)
	defer eng.Stop()

	assert.Equal(t, 4, eng.ConnectionCount())
}

func TestEngine_SetConnections_ScaleUp(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{Connections: 2})
	err := eng.Start(engine.ModeDownload)
	require.NoError(t, err)
	defer eng.Stop()

	assert.Equal(t, 2, eng.ConnectionCount())

	eng.SetConnections(5)
	assert.Equal(t, 5, eng.ConnectionCount())

	// Give new workers time to register and verify they're active
	time.Sleep(200 * time.Millisecond)
	assert.Greater(t, eng.Status().Snapshot.ActiveConnections, 0)
}

func TestEngine_SetConnections_ScaleDown(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{Connections: 6})
	err := eng.Start(engine.ModeDownload)
	require.NoError(t, err)
	defer eng.Stop()

	assert.Equal(t, 6, eng.ConnectionCount())

	eng.SetConnections(3)
	assert.Equal(t, 3, eng.ConnectionCount())
}

func TestEngine_SetConnections_WhilePaused_Noop(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{Connections: 4})
	err := eng.Start(engine.ModeDownload)
	require.NoError(t, err)
	defer eng.Stop()

	eng.Pause()
	assert.True(t, eng.IsPaused())
	assert.Equal(t, 0, eng.ConnectionCount())

	eng.SetConnections(10)
	assert.Equal(t, 0, eng.ConnectionCount(), "SetConnections should be a no-op while paused")
}
