package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"vpnkit/internal/netx"
)

// Asset is one GitHub release artifact.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is a subset of GitHub's release schema sufficient for our needs.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// AssetURL returns the URL of the asset whose name matches exactly.
func (r Release) AssetURL(name string) (string, error) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL, nil
		}
	}
	return "", fmt.Errorf("asset %q not found in release %s", name, r.Tag)
}

// ReleaseClient queries the GitHub Releases API.
type ReleaseClient struct {
	BaseURL string // e.g. "https://api.github.com"
	Token   string // optional GITHUB_TOKEN; empty => unauthenticated
	HTTP    *http.Client
}

// NewReleaseClient constructs a client with a 10s timeout that honors the
// user's env proxy (HTTPS_PROXY / HTTP_PROXY) when set. Bootstrap callers
// that must avoid the v0.9.x deadlock (env proxy is vpnkit's own
// not-yet-running mihomo) should override .HTTP with noProxyHTTPClient()
// after construction.
func NewReleaseClient(baseURL, token string) *ReleaseClient {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &ReleaseClient{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    netx.SmartClient(10 * time.Second),
	}
}

// noProxyHTTPClient is the bootstrap-safe client (Transport.Proxy=nil).
// Used by Install() when Options.NoProxy is true.
func noProxyHTTPClient() *http.Client {
	return netx.NoProxyClient(10 * time.Second)
}

// Latest fetches MetaCubeX/mihomo's latest release.
func (c *ReleaseClient) Latest() (Release, error) {
	return c.LatestForRepo("MetaCubeX/mihomo")
}

// ByTag fetches a specific tagged release of MetaCubeX/mihomo.
func (c *ReleaseClient) ByTag(tag string) (Release, error) {
	return c.ByTagForRepo("MetaCubeX/mihomo", tag)
}

// LatestForRepo fetches `<owner>/<repo>`'s latest non-prerelease release.
func (c *ReleaseClient) LatestForRepo(repo string) (Release, error) {
	return c.byPath(fmt.Sprintf("/repos/%s/releases/latest", repo))
}

// ByTagForRepo fetches a specific tag.
func (c *ReleaseClient) ByTagForRepo(repo, tag string) (Release, error) {
	return c.byPath(fmt.Sprintf("/repos/%s/releases/tags/%s", repo, tag))
}

func (c *ReleaseClient) byPath(path string) (Release, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github: %s", resp.Status)
	}
	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Release{}, err
	}
	return r, nil
}

