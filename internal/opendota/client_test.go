package opendota

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientEndpoints(t *testing.T) {
	var parseRequested bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "secret" {
			t.Errorf("missing API key")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/players/256161923/recentMatches":
			_, _ = w.Write([]byte(`[{"match_id":42,"hero_id":93}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/matches/42":
			_, _ = w.Write([]byte(`{"match_id":42,"cluster":123,"replay_salt":999}`))
		case r.Method == http.MethodPost && r.URL.Path == "/request/42":
			parseRequested = true
			_, _ = w.Write([]byte(`{"job":{"jobId":"x"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret", server.Client())
	recent, err := client.RecentMatches(context.Background(), 256161923)
	if err != nil || len(recent) != 1 || recent[0].MatchID != 42 {
		t.Fatalf("recent = %#v, err = %v", recent, err)
	}
	match, err := client.Match(context.Background(), 42)
	if err != nil || match.ReplaySalt != 999 {
		t.Fatalf("match = %#v, err = %v", match, err)
	}
	if err := client.RequestParse(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if !parseRequested {
		t.Fatal("parse was not requested")
	}
}

func TestClientReturnsStatusErrorWithoutLeakingAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClient(server.URL, "very-secret", server.Client())
	_, err := client.Match(context.Background(), 42)
	if err == nil || !IsStatus(err, http.StatusTooManyRequests) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "very-secret") {
		t.Fatalf("error leaked API key: %v", err)
	}
}
