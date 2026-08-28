package opendota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const userAgent = "git-gud/0.1"

type HTTPStatusError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("opendota %s %s: status %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

func IsStatus(err error, status int) bool {
	var statusErr *HTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == status
}

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  strings.TrimSpace(apiKey),
		http:    httpClient,
	}
}

func safeURL(u *url.URL) string {
	clone := *u
	q := clone.Query()
	if q.Has("api_key") {
		q.Set("api_key", "REDACTED")
		clone.RawQuery = q.Encode()
	}
	return clone.String()
}

func (c *Client) RecentMatches(ctx context.Context, accountID uint32) ([]RecentMatch, error) {
	var out []RecentMatch
	path := "/players/" + strconv.FormatUint(uint64(accountID), 10) + "/recentMatches"
	if err := c.doJSON(ctx, http.MethodGet, path, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) Match(ctx context.Context, matchID int64) (Match, error) {
	var out Match
	path := "/matches/" + strconv.FormatInt(matchID, 10)
	if err := c.doJSON(ctx, http.MethodGet, path, &out); err != nil {
		return Match{}, err
	}
	return out, nil
}

func (c *Client) RequestParse(ctx context.Context, matchID int64) error {
	path := "/request/" + strconv.FormatInt(matchID, 10)
	return c.doJSON(ctx, http.MethodPost, path, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, out any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("build opendota URL: %w", err)
	}
	if c.apiKey != "" {
		q := u.Query()
		q.Set("api_key", c.apiKey)
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build opendota request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("opendota request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return &HTTPStatusError{
			Method:     method,
			URL:        safeURL(u),
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode opendota response: %w", err)
	}
	return nil
}
