package sources_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/ayush18/networkbooster/core/sources"
	"github.com/ayush18/networkbooster/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfHostedSource_Name(t *testing.T) {
	sh := sources.NewSelfHostedSource("http://myserver.com")
	assert.Equal(t, "Self-Hosted", sh.Name())
	assert.Equal(t, sources.SourceTypeSelfHosted, sh.Type())
}

func TestSelfHostedSource_Discover(t *testing.T) {
	sh := sources.NewSelfHostedSource("http://myserver.com")
	servers, err := sh.Discover()
	require.NoError(t, err)
	assert.Len(t, servers, 1)
	assert.Equal(t, "http://myserver.com", servers[0].URL)
}

func TestSelfHostedSource_Download(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	sh := sources.NewSelfHostedSource(ts.URL)
	server := sources.Server{ID: "self", Name: "Self", URL: ts.URL + "/download"}

	rc, err := sh.Download(context.Background(), server)
	require.NoError(t, err)
	defer rc.Close()

	n, err := io.Copy(io.Discard, rc)
	require.NoError(t, err)
	assert.Greater(t, n, int64(0))
}

func TestSelfHostedSource_Upload(t *testing.T) {
	ts := testutil.NewTestServer()
	defer ts.Close()

	sh := sources.NewSelfHostedSource(ts.URL)
	server := sources.Server{ID: "self", Name: "Self", URL: ts.URL + "/upload"}

	data := bytes.NewReader(make([]byte, 1024))
	err := sh.Upload(context.Background(), server, data)
	require.NoError(t, err)
}
