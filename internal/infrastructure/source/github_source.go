package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Franogales/scalable_code_ai/internal/domain"
)

const (
	DefaultRepo        = "Franogales/scalable_code_ai"
	DefaultRef         = "main"
	DefaultRawBaseURL  = "https://raw.githubusercontent.com"
	manifestRelPath    = "manifest.json"
	defaultHTTPTimeout = 30 * time.Second
)

// GitHubSource fetches registry files from GitHub raw content.
type GitHubSource struct {
	baseURL string
	client  *http.Client
}

// NewGitHubSource constructs an AssetSource pointing to https://raw.githubusercontent.com/<repo>/<ref>/registry.
func NewGitHubSource(repo, ref string) *GitHubSource {
	if repo == "" {
		repo = DefaultRepo
	}
	if ref == "" {
		ref = DefaultRef
	}
	baseURL := fmt.Sprintf("%s/%s/%s/registry", DefaultRawBaseURL, strings.Trim(repo, "/"), strings.Trim(ref, "/"))
	return &GitHubSource{
		baseURL: baseURL,
		client:  &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// NewGitHubSourceWithBaseURL allows overriding the base URL (useful for testing).
func NewGitHubSourceWithBaseURL(baseURL string, client *http.Client) *GitHubSource {
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &GitHubSource{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

// FetchManifest downloads and decodes registry/manifest.json.
func (s *GitHubSource) FetchManifest(ctx context.Context) (domain.Manifest, error) {
	data, err := s.FetchFile(ctx, manifestRelPath)
	if err != nil {
		return domain.Manifest{}, fmt.Errorf("fetch manifest from github: %w", err)
	}
	var manifest domain.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return domain.Manifest{}, fmt.Errorf("decode manifest.json: %w", err)
	}
	return manifest, nil
}

// FetchFile downloads a relative file inside the registry directory.
func (s *GitHubSource) FetchFile(ctx context.Context, relativePath string) ([]byte, error) {
	cleanPath, err := sanitizeRelativePath(relativePath)
	if err != nil {
		return nil, err
	}

	url := s.baseURL + "/" + cleanPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request for %s: %w", url, err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s returned HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response for %s: %w", url, err)
	}
	return body, nil
}

func sanitizeRelativePath(relativePath string) (string, error) {
	slashPath := strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/")
	if slashPath == "" || strings.HasPrefix(slashPath, "/") {
		return "", fmt.Errorf("invalid relative path %q", relativePath)
	}
	cleaned := path.Clean(slashPath)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("unsafe relative path %q", relativePath)
	}
	return cleaned, nil
}
