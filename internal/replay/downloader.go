package replay

import (
	"compress/bzip2"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ap0calypse644/git_gud/internal/opendota"
)

type HTTPStatusError struct {
	URL        string
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("replay download %s: status %d", e.URL, e.StatusCode)
}

func IsStatus(err error, status int) bool {
	var statusErr *HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == status
}

type Downloader struct {
	http           *http.Client
	root           string
	keepCompressed bool
}

func NewDownloader(httpClient *http.Client, storageRoot string, keepCompressed bool) *Downloader {
	return &Downloader{http: httpClient, root: storageRoot, keepCompressed: keepCompressed}
}

func ResolveURL(match opendota.Match) string {
	if strings.TrimSpace(match.ReplayURL) != "" {
		return strings.TrimSpace(match.ReplayURL)
	}
	if match.Cluster > 0 && match.ReplaySalt > 0 {
		return fmt.Sprintf("https://replay%d.valve.net/570/%d_%d.dem.bz2", match.Cluster, match.MatchID, match.ReplaySalt)
	}
	return ""
}

func (d *Downloader) Acquire(ctx context.Context, match opendota.Match) (string, error) {
	replayURL := ResolveURL(match)
	if replayURL == "" {
		return "", errors.New("match has no replay URL or replay salt")
	}

	dir := filepath.Join(d.root, "replays")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create replay directory: %w", err)
	}
	finalDEM := filepath.Join(dir, fmt.Sprintf("%d.dem", match.MatchID))
	if info, err := os.Stat(finalDEM); err == nil && info.Size() > 0 {
		return finalDEM, nil
	}

	compressedPath := filepath.Join(dir, fmt.Sprintf("%d.dem.bz2", match.MatchID))
	if _, err := os.Stat(compressedPath); errors.Is(err, os.ErrNotExist) {
		if err := d.download(ctx, replayURL, compressedPath); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", fmt.Errorf("stat compressed replay: %w", err)
	}

	if err := decompressBzip2(compressedPath, finalDEM); err != nil {
		// A truncated/corrupt completed download must not poison every future retry.
		_ = os.Remove(compressedPath)
		return "", err
	}
	if !d.keepCompressed {
		_ = os.Remove(compressedPath)
	}
	return finalDEM, nil
}

func (d *Downloader) download(ctx context.Context, rawURL, destination string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse replay URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported replay URL scheme %q", u.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build replay request: %w", err)
	}
	req.Header.Set("User-Agent", "git-gud/0.1")
	resp, err := d.http.Do(req)
	if err != nil {
		return fmt.Errorf("download replay: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPStatusError{URL: u.Redacted(), StatusCode: resp.StatusCode}
	}

	tmp, err := os.CreateTemp(filepath.Dir(destination), ".replay-*.bz2.part")
	if err != nil {
		return fmt.Errorf("create replay temp file: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return fmt.Errorf("write replay: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync replay: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close replay: %w", err)
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return fmt.Errorf("commit replay download: %w", err)
	}
	ok = true
	return nil
}

func decompressBzip2(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open compressed replay: %w", err)
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(destination), ".replay-*.dem.part")
	if err != nil {
		return fmt.Errorf("create decompressed replay temp file: %w", err)
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, bzip2.NewReader(in)); err != nil {
		return fmt.Errorf("decompress replay: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync decompressed replay: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close decompressed replay: %w", err)
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return fmt.Errorf("commit decompressed replay: %w", err)
	}
	ok = true
	return nil
}
