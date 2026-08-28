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
	"github.com/klauspost/compress/zstd"
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

func TestAcquireDownloadsAndDecompressesBzip2(t *testing.T) {
	// bzip2-compressed bytes for: PBDEMS2\x00test replay\n
	compressed, err := base64.StdEncoding.DecodeString("QlpoOTFBWSZTWZvICtIAAARfgEAQQAAQABYCSAAiBFwgIAAigB6nqDyhTTIxMTEo2TkaB6gl6TT+LuSKcKEhN5AVpA==")
	if err != nil {
		t.Fatal(err)
	}
	assertAcquirePayload(t, compressed, "test replay")
}

func TestAcquireDownloadsAndDecompressesZstd(t *testing.T) {
	payload := []byte("PBDEMS2\x00modern zstd replay\n")
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	compressed := encoder.EncodeAll(payload, nil)
	encoder.Close()
	assertAcquirePayload(t, compressed, "modern zstd replay")
}

func TestAcquireAcceptsAlreadyUncompressedDEM(t *testing.T) {
	assertAcquirePayload(t, []byte("PBDEMS2\x00raw replay\n"), "raw replay")
}

func TestAcquireRejectsUnknownPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-a-replay"))
	}))
	defer server.Close()

	root := t.TempDir()
	d := NewDownloader(server.Client(), root, false)
	_, err := d.Acquire(context.Background(), opendota.Match{MatchID: 99, ReplayURL: server.URL + "/99.dem.bz2"})
	if err == nil || !strings.Contains(err.Error(), "unknown replay compression/header") {
		t.Fatalf("Acquire error = %v", err)
	}
	if _, err := os.Stat(root + "/replays/99.dem.download"); !os.IsNotExist(err) {
		t.Fatalf("bad download should be removed, stat err = %v", err)
	}
}

func assertAcquirePayload(t *testing.T, served []byte, wantSubstring string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(served)
	}))
	defer server.Close()

	root := t.TempDir()
	d := NewDownloader(server.Client(), root, false)
	path, err := d.Acquire(context.Background(), opendota.Match{MatchID: 42, ReplayURL: server.URL + "/42.dem.bz2"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), wantSubstring) {
		t.Fatalf("decompressed payload = %q", data)
	}
	if _, err := os.Stat(root + "/replays/42.dem.download"); !os.IsNotExist(err) {
		t.Fatalf("downloaded replay should be removed, stat err = %v", err)
	}
}
