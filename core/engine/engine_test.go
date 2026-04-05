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

func TestEngine_StartDownload(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{
		Connections: 4,
	})

	err := eng.Start(engine.ModeDownload)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	status := eng.Status()
	assert.True(t, status.Running)
	assert.Greater(t, status.Snapshot.TotalDownloadBytes, int64(0))
	assert.Greater(t, status.Snapshot.ActiveConnections, 0)

	err = eng.Stop()
	require.NoError(t, err)

	status = eng.Status()
	assert.False(t, status.Running)
}

func TestEngine_StartUpload(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/upload")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{
		Connections: 2,
	})

	err := eng.Start(engine.ModeUpload)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	status := eng.Status()
	assert.True(t, status.Running)
	assert.Greater(t, status.Snapshot.TotalUploadBytes, int64(0))

	err = eng.Stop()
	require.NoError(t, err)
}

func TestEngine_StartBidirectional(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL)
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{
		Connections: 2,
	})

	err := eng.Start(engine.ModeBidirectional)
	require.NoError(t, err)

	time.Sleep(300 * time.Millisecond)

	status := eng.Status()
	assert.True(t, status.Running)
	assert.Greater(t, status.Snapshot.TotalDownloadBytes, int64(0))
	assert.Greater(t, status.Snapshot.TotalUploadBytes, int64(0))

	eng.Stop()
}

func TestEngine_DoubleStartReturnsError(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	reg := sources.NewRegistry()
	sh := sources.NewSelfHostedSource(ts.URL + "/download")
	reg.Register(sh)

	eng := engine.New(reg, engine.Options{Connections: 1})
	eng.Start(engine.ModeDownload)
	defer eng.Stop()

	err := eng.Start(engine.ModeDownload)
	assert.Error(t, err, "starting an already running engine should error")
}
