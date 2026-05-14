package discovery

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// ManifestEntry is a single plugin entry from plugin-manifest.json.
type ManifestEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	Repo         string `json:"repo"`
	TestPassed   bool   `json:"test_passed"`
	LastCI       string `json:"last_ci"`
}

// Manifest is the top-level structure of plugin-manifest.json.
type Manifest struct {
	Plugins []ManifestEntry `json:"plugins"`
}

// Fetcher pulls the plugin manifest from a URL (GitHub raw, etc.).
type Fetcher struct {
	URL    string
	client *http.Client
}

// NewFetcher creates a Fetcher for the given manifest URL.
func NewFetcher(url string) *Fetcher {
	return &Fetcher{
		URL: url,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Fetch pulls and parses the manifest.
func (f *Fetcher) Fetch() (*Manifest, error) {
	log.Printf("🔍 Fetching plugin manifest from %s", f.URL)

	resp, err := f.client.Get(f.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read manifest body: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest JSON: %w", err)
	}

	log.Printf("   ✅ Found %d plugin(s) in manifest", len(manifest.Plugins))
	return &manifest, nil
}
