package registry

import (
	"encoding/json"
	"fmt"
	"sync"
)

// State represents a plugin's lifecycle state.
type State string

const (
	StateUnloaded State = "unloaded"
	StateLoaded   State = "loaded"
	StateEnabled  State = "enabled"
	StateDisabled State = "disabled"
	StateError    State = "error"
)

// Plugin holds a plugin's runtime state.
type Plugin struct {
	ID           string      `json:"plugin_id"`
	Name         string      `json:"plugin_name"`
	Version      string      `json:"version"`
	Description  string      `json:"description"`
	Repo         string      `json:"repo"`
	State        State       `json:"state"`
	TestPassed   bool        `json:"test_passed"`
	TestSummary  string      `json:"test_summary,omitempty"`
	TestResults  []TestResult `json:"test_results,omitempty"`
	RouteTarget  string      `json:"route_target"`
}

// TestResult is a single test case outcome.
type TestResult struct {
	Name       string  `json:"name"`
	Passed     bool    `json:"passed"`
	DurationMs float64 `json:"duration_ms"`
}

// Registry is the thread-safe in-memory plugin registry.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*Plugin
}

// New creates a new empty Registry.
func New() *Registry {
	return &Registry{plugins: make(map[string]*Plugin)}
}

// LoadFromManifest registers a plugin from a manifest entry.
// TestPassed is set from CI results, not runtime testing.
func (r *Registry) LoadFromManifest(id, name, version, desc, repo string, testPassed bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[id]; exists {
		// Update existing plugin's test status from manifest
		r.plugins[id].TestPassed = testPassed
		return
	}

	state := StateLoaded
	if testPassed {
		state = StateLoaded
	}

	r.plugins[id] = &Plugin{
		ID:          id,
		Name:        name,
		Version:     version,
		Description: desc,
		Repo:        repo,
		State:       state,
		TestPassed:  testPassed,
		RouteTarget: "default",
	}
}

// Enable marks a plugin as enabled if it has passed tests.
func (r *Registry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	if !p.TestPassed {
		return fmt.Errorf("plugin %q has not passed tests", id)
	}
	if p.State == StateEnabled {
		return nil
	}
	p.State = StateEnabled
	return nil
}

// Disable marks a plugin as disabled.
func (r *Registry) Disable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	if p.State != StateEnabled {
		return nil
	}
	p.State = StateDisabled
	return nil
}

// RecordTest stores test results for a plugin.
func (r *Registry) RecordTest(id string, passed bool, summary string, results []TestResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.plugins[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	p.TestPassed = passed
	p.TestSummary = summary
	p.TestResults = results
	return nil
}

// Get returns a plugin by ID.
func (r *Registry) Get(id string) (*Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[id]
	return p, ok
}

// ListAll returns all registered plugins.
func (r *Registry) ListAll() []*Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		list = append(list, p)
	}
	return list
}

// EnabledCount returns count of enabled plugins.
func (r *Registry) EnabledCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n := 0
	for _, p := range r.plugins {
		if p.State == StateEnabled {
			n++
		}
	}
	return n
}

// EnabledVendors returns the IDs of all enabled plugins.
func (r *Registry) EnabledVendors() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var vendors []string
	for id, p := range r.plugins {
		if p.State == StateEnabled {
			vendors = append(vendors, id)
		}
	}
	return vendors
}

// Stats returns a JSON-serializable health summary.
func (r *Registry) Stats() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]any{
		"status":          "healthy",
		"plugins_total":   len(r.plugins),
		"plugins_enabled": r.EnabledCount(),
	}
}

// MustJSON is a helper for handlers.
func MustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
