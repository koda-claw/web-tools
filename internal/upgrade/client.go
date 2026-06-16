package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

// HTTPClient performs release network operations.
type HTTPClient interface {
	GetBytes(ctx context.Context, rawURL string) ([]byte, error)
	DownloadFile(ctx context.Context, rawURL string, dst string) error
}

// NetClient is the default HTTP-backed client.
type NetClient struct {
	Client *http.Client
}

func (c NetClient) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c NetClient) GetBytes(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
}

func (c NetClient) DownloadFile(ctx context.Context, rawURL string, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, io.LimitReader(resp.Body, 256*1024*1024))
	return err
}

type releaseClient struct {
	client HTTPClient
}

func newReleaseClient(client HTTPClient) releaseClient {
	if client == nil {
		client = NetClient{}
	}
	return releaseClient{client: client}
}

func (c releaseClient) resolveLatest(ctx context.Context, repo string) (string, error) {
	rawURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	data, err := c.client.GetBytes(ctx, rawURL)
	if err != nil {
		return "", apperrors.NewNetworkError(
			"cannot resolve latest release",
			err.Error(),
			map[string]string{"url": rawURL},
			[]string{"check network access", "use --version vX.Y.Z to avoid latest lookup"},
		)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", apperrors.NewInputError("cannot parse latest release", err.Error(), []string{"use --version vX.Y.Z"})
	}
	if payload.TagName == "" {
		return "", apperrors.NewInputError("latest release has no tag", rawURL, []string{"use --version vX.Y.Z"})
	}
	return payload.TagName, nil
}

func releaseAssetURL(repo, baseURL, tag, asset string) string {
	if baseURL != "" {
		return joinURL(baseURL, tag, asset)
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, asset)
}

func checksumURL(repo, baseURL, tag string) string {
	if baseURL != "" {
		return joinURL(baseURL, tag, "checksums.txt")
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/checksums.txt", repo, tag)
}

func joinURL(base string, parts ...string) string {
	base = strings.TrimRight(base, "/")
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(strings.Trim(part, "/")))
	}
	return base + "/" + strings.Join(escaped, "/")
}
