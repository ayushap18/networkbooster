package sources_test

import (
	"context"
	"io"
	"testing"

	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCDNSource_Name(t *testing.T) {
	cdn := sources.NewCDNSource()
	assert.Equal(t, "CDN Mirrors", cdn.Name())
	assert.Equal(t, sources.SourceTypeCDN, cdn.Type())
}

func TestCDNSource_Discover(t *testing.T) {
	cdn := sources.NewCDNSource()
	servers, err := cdn.Discover()
	require.NoError(t, err)
	assert.Greater(t, len(servers), 0, "should have at least one CDN server")
	for _, s := range servers {
		assert.NotEmpty(t, s.ID)
		assert.NotEmpty(t, s.URL)
	}
}

func TestCDNSource_Download(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	cdn := sources.NewCDNSource()
	server := sources.Server{ID: "test", Name: "Test", URL: ts.URL + "/download"}

	rc, err := cdn.Download(context.Background(), server)
	require.NoError(t, err)
	defer rc.Close()

	n, err := io.Copy(io.Discard, rc)
	require.NoError(t, err)
	assert.Greater(t, n, int64(0), "should have downloaded some data")
}

func TestCDNSource_Upload_NotSupported(t *testing.T) {
	cdn := sources.NewCDNSource()
	server := sources.Server{ID: "test", Name: "Test", URL: "http://example.com"}

	err := cdn.Upload(context.Background(), server, nil)
	assert.Error(t, err, "CDN should not support upload")
}
