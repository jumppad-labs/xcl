package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	xclerrors "github.com/jumppad-labs/xcl/errors"
)

// releaseClient reads releases and downloads their assets from the GitHub
// REST API at baseURL, sending token with every request when there is one
type releaseClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// githubRelease is the part of GitHub's release JSON the registry reads
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Draft   bool          `json:"draft"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset is one file attached to a release. URL is the asset's API
// endpoint, which serves its bytes to a request accepting
// application/octet-stream, for private repositories too.
type githubAsset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// asset returns the release's asset with exactly name
func (r *githubRelease) asset(name string) (githubAsset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}

	return githubAsset{}, false
}

// release reads the release of repository (owner/repo) tagged tag. A tag with
// no release, or a draft release, is ErrPluginNotFound. GitHub answers a
// request for a private repository without a token as not found, so without a
// token the error says the repository may need one.
func (c *releaseClient) release(ctx context.Context, repository, tag string) (*githubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/tags/%s", c.baseURL, repository, tag)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Accept", "application/vnd.github+json")

	response, err := c.do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound && c.token == "" {
		return nil, fmt.Errorf(
			"%w: %s release %s was not found, or the repository is private and needs a GitHub token (set GITHUB_TOKEN)",
			xclerrors.ErrPluginNotFound, repository, tag,
		)
	}

	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: no release %s in %s", xclerrors.ErrPluginNotFound, tag, repository)
	}

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		if c.token != "" {
			return nil, fmt.Errorf("unable to read release %s of %s, the GitHub token was rejected: %w", tag, repository, statusError(response))
		}
	}

	if err := statusError(response); err != nil {
		return nil, fmt.Errorf("unable to read release %s of %s: %w", tag, repository, err)
	}

	var found githubRelease
	if err := json.NewDecoder(response.Body).Decode(&found); err != nil {
		return nil, fmt.Errorf("unable to read release %s of %s: %w", tag, repository, err)
	}

	if found.Draft {
		return nil, fmt.Errorf("%w: release %s of %s is a draft", xclerrors.ErrPluginNotFound, tag, repository)
	}

	return &found, nil
}

// download writes the bytes of asset to w
func (c *releaseClient) download(ctx context.Context, asset githubAsset, w io.Writer) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}

	request.Header.Set("Accept", "application/octet-stream")

	response, err := c.do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if err := statusError(response); err != nil {
		return fmt.Errorf("unable to download %s: %w", asset.Name, err)
	}

	if _, err := io.Copy(w, response.Body); err != nil {
		return fmt.Errorf("unable to download %s: %w", asset.Name, err)
	}

	return nil
}

// do sends request with the headers every GitHub API request carries
func (c *releaseClient) do(request *http.Request) (*http.Response, error) {
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "xcl")

	// Go drops this header when a download is redirected to another host,
	// GitHub's asset storage URLs are pre-signed and must not receive it
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	client := c.http
	if client == nil {
		client = http.DefaultClient
	}

	return client.Do(request)
}

// statusError is an error naming response's status and the start of its
// body, or nil for a 2xx response
func statusError(response *http.Response) error {
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))

	return fmt.Errorf("GitHub answered %s: %s", response.Status, strings.TrimSpace(string(body)))
}

// downloadTo downloads asset into a new file at path
func downloadTo(ctx context.Context, client *releaseClient, asset githubAsset, path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if err := client.download(ctx, asset, file); err != nil {
		file.Close()
		return err
	}

	return file.Close()
}
