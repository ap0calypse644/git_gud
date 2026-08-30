package replay

import (
	"bytes"
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
	"github.com/klauspost/compress/zstd"
)

var (
	rawDEMMagic = []byte{'P', 'B', 'D', 'E', 'M', 'S', '2', 0}
	zstdMagic   = []byte{0x28, 0xB5, 0x2F, 0xFD}
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
	if info, err := os.Stat(finalDEM); err == nil {
		if info.Mode().IsRegular() && info.Size() > 0 {
			if err := validateReplayFile(finalDEM); err == nil {
				return finalDEM, nil
			}
		}
		// A stale zero-length, truncated, or otherwise invalid cached .dem must
		// not be trusted forever. Remove it and reacquire from the replay host.
		if err := os.Remove(finalDEM); err != nil {
			return "", fmt.Errorf("remove invalid cached replay: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat cached replay: %w", err)
	}

	// Valve historically served bzip2 under a .dem.bz2 URL, but modern
	// replay servers can return Zstandard while retaining that URL suffix.
	// Store the raw response under a format-neutral name and detect its
	// contents by magic bytes before decompression.
	downloadedPath := filepath.Join(dir, fmt.Sprintf("%d.dem.download", match.MatchID))
	if _, err := os.Stat(downloadedPath); errors.Is(err, os.ErrNotExist) {
		if err := d.download(ctx, replayURL, downloadedPath); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", fmt.Errorf("stat downloaded replay: %w", err)
	}

	if err := decompressReplay(downloadedPath, finalDEM); err != nil {
		// A truncated, corrupt, or unexpected completed download must not
		// poison every future retry.
		_ = os.Remove(downloadedPath)
		return "", err
	}
	if !d.keepCompressed {
		_ = os.Remove(downloadedPath)
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

	tmp, err := os.CreateTemp(filepath.Dir(destination), ".replay-*.download.part")
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

func decompressReplay(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open downloaded replay: %w", err)
	}
	defer in.Close()

	header := make([]byte, len(rawDEMMagic))
	n, err := io.ReadFull(in, header)
	if err != nil {
		return fmt.Errorf("read replay header: %w", err)
	}
	header = header[:n]
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind replay: %w", err)
	}

	var reader io.Reader
	var closeReader func()
	switch {
	case bytes.HasPrefix(header, rawDEMMagic):
		reader = in
	case len(header) >= 4 && header[0] == 'B' && header[1] == 'Z' && header[2] == 'h' && header[3] >= '1' && header[3] <= '9':
		reader = bzip2.NewReader(in)
	case bytes.HasPrefix(header, zstdMagic):
		decoder, err := zstd.NewReader(in)
		if err != nil {
			return fmt.Errorf("open zstd replay: %w", err)
		}
		reader = decoder
		closeReader = decoder.Close
	default:
		return fmt.Errorf("unknown replay compression/header: % X", header)
	}
	if closeReader != nil {
		defer closeReader()
	}

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

	if _, err := io.Copy(tmp, reader); err != nil {
		return fmt.Errorf("decompress replay: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync decompressed replay: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close decompressed replay: %w", err)
	}

	// A successful decompression is not enough: the payload must actually be
	// a Source 2 Dota demo. This catches HTML/error payloads that happened to
	// arrive with HTTP 200 and protects Manta from confusing downstream errors.
	if err := validateReplayFile(tmpName); err != nil {
		return fmt.Errorf("verify decompressed replay: %w", err)
	}

	if err := os.Rename(tmpName, destination); err != nil {
		return fmt.Errorf("commit decompressed replay: %w", err)
	}
	ok = true
	return nil
}

func validateReplayFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open replay: %w", err)
	}
	defer f.Close()

	magic := make([]byte, len(rawDEMMagic))
	if _, err := io.ReadFull(f, magic); err != nil {
		return fmt.Errorf("read replay header: %w", err)
	}
	if !bytes.Equal(magic, rawDEMMagic) {
		return fmt.Errorf("payload is not a Source 2 replay: header % X", magic)
	}
	return nil
}
