package replay

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/ap0calypse644/git_gud/internal/opendota"
)

func TestResolveURL(t *testing.T) {
	m := opendota.Match{MatchID: 42, Cluster: 123, ReplaySalt: 999}
	got := ResolveURL(m)
	want := "https://replay123.valve.net/570/42_999.dem.bz2"
	if got != want {
		t.Fatalf("ResolveURL = %q, want %q", got, want)
	}
	m.ReplayURL = "https://example.test/replay.dem.bz2"
	if got := ResolveURL(m); got != m.ReplayURL {
		t.Fatalf("explicit replay URL not preferred: %q", got)
	}
}

func TestAcquireDownloadsAndDecompresses(t *testing.T) {
	// bzip2-compressed bytes for: PBDEMS2\x00test replay\n
	compressed, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWZvICtIAAARfgEAQQAAQABYCSAAiBFwgIAAigB6nqDyhTTIxMTEo2TkaB6gl6TT+LuSKcKEhN5AVpA==")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(compressed)
	}))
	defer server.Close()

	d := NewDownloader(server.Client(), t.TempDir(), false)
	path, err := d.Acquire(context.Background(), opendota.Match{MatchID: 42, ReplayURL: server.URL + "/42.dem.bz2"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "test replay") {
		t.Fatalf("decompressed payload = %q", data)
	}
	if _, err := os.Stat(strings.TrimSuffix(path, ".dem") + ".dem.bz2"); !os.IsNotExist(err) {
		t.Fatalf("compressed replay should be removed, stat err = %v", err)
	}
}
