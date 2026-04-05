package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/ayush18/networkbooster/core/engine"
	"github.com/ayush18/networkbooster/core/metrics"
	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestDownloadWorker_TransfersData(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	collector := metrics.NewCollector()
	cdn := sources.NewCDNSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/download"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	w := engine.NewWorker("w-1", cdn, server, collector)
	w.RunDownload(ctx)

	snap := collector.Snapshot()
	assert.Greater(t, snap.TotalDownloadBytes, int64(0), "worker should have downloaded data")
}

func TestUploadWorker_TransfersData(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	collector := metrics.NewCollector()
	sh := sources.NewSelfHostedSource(ts.URL)
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/upload"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	w := engine.NewWorker("w-1", sh, server, collector)
	w.RunUpload(ctx)

	snap := collector.Snapshot()
	assert.Greater(t, snap.TotalUploadBytes, int64(0), "worker should have uploaded data")
}
