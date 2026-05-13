package handler

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/shawnlin0125/plugin-hub/internal/registry"
)

//go:embed dashboard.html
var dashboardHTML embed.FS

var dashTmpl *template.Template

func init() {
	data, err := dashboardHTML.ReadFile("dashboard.html")
	if err != nil {
		panic("dashboard.html not embedded: " + err.Error())
	}
	dashTmpl = template.Must(template.New("dashboard").Parse(string(data)))
}

// Handler bundles all HTTP handlers with a registry.
type Handler struct {
	Reg *registry.Registry
}

// New creates a Handler.
func New(reg *registry.Registry) *Handler {
	return &Handler{Reg: reg}
}

// ── Dashboard ────────────────────────────────────────────────────────

// Dashboard serves the SPA HTML page.
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashTmpl.Execute(w, nil); err != nil {
		http.Error(w, "template error", 500)
	}
}

// ── Health ───────────────────────────────────────────────────────────

// Health returns platform health status.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.Reg.Stats())
}

// ── Plugin CRUD ──────────────────────────────────────────────────────

// ListPlugins returns all discovered plugins.
func (h *Handler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.Reg.ListAll())
}

// GetPlugin returns a single plugin's details.
func (h *Handler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := h.Reg.Get(id)
	if !ok {
		http.Error(w, `{"error":"plugin not found"}`, 404)
		return
	}
	writeJSON(w, p)
}

// EnablePlugin enables a plugin (requires test passed).
func (h *Handler) EnablePlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Reg.Enable(id); err != nil {
		writeJSONStatus(w, 403, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("✅ Plugin %q enabled", id)
	writeJSON(w, map[string]string{"status": "enabled", "plugin_id": id})
}

// DisablePlugin disables a running plugin.
func (h *Handler) DisablePlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Reg.Disable(id); err != nil {
		writeJSONStatus(w, 403, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("⏹  Plugin %q disabled", id)
	writeJSON(w, map[string]string{"status": "disabled", "plugin_id": id})
}

// ── Test Gate ────────────────────────────────────────────────────────

// TestPlugin runs the plugin's test command and records results.
func (h *Handler) TestPlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := h.Reg.Get(id)
	if !ok {
		http.Error(w, `{"error":"plugin not found"}`, 404)
		return
	}

	// Read test_command from plugin config (passed via env or stored)
	// For now, derive from plugin ID — in practice this comes from config
	testCmd := fmt.Sprintf("pytest %s -v --json-report 2>/dev/null || echo '{\"summary\":{\"passed\":0,\"total\":0}}'", id)

	// Try to run the test command
	log.Printf("🧪 Running tests for %q: %s", id, testCmd)
	start := time.Now()

	cmd := exec.Command("sh", "-c", testCmd)
	cmd.Dir = "/app" // plugin code would be mounted here
	output, err := cmd.CombinedOutput()

	elapsed := time.Since(start)
	outputStr := string(output)

	// If test command fails or doesn't exist, return a helpful message
	if err != nil && strings.Contains(outputStr, "not found") {
		writeJSON(w, map[string]any{
			"plugin_id": id,
			"passed":    false,
			"summary":   "Test runner not available (run tests in CI: " + p.Repo + "/actions)",
			"results":   []registry.TestResult{},
			"elapsed_ms": elapsed.Milliseconds(),
		})
		return
	}

	// Parse pytest JSON output if available, otherwise count pass/fail from output
	passed := false
	summary := "0/0 passed"
	results := []registry.TestResult{}

	// Try JSON parsing first (pytest-json-report)
	if strings.Contains(outputStr, "\"summary\"") {
		var report struct {
			Summary struct {
				Passed int `json:"passed"`
				Total  int `json:"total"`
			} `json:"summary"`
			Tests []struct {
				Outcome  string  `json:"outcome"`
				NodeID   string  `json:"nodeid"`
				Duration float64 `json:"duration"`
			} `json:"tests"`
		}
		if json.Unmarshal(output, &report) == nil && report.Summary.Total > 0 {
			passed = report.Summary.Passed == report.Summary.Total
			summary = fmt.Sprintf("%d/%d passed", report.Summary.Passed, report.Summary.Total)
			for _, t := range report.Tests {
				results = append(results, registry.TestResult{
					Name:       t.NodeID,
					Passed:     t.Outcome == "passed",
					DurationMs: t.Duration * 1000,
				})
			}
		}
	} else {
		// Fallback: scan for pytest summary line
		// e.g. "6 passed, 3 failed" or "12 passed"
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "passed") || strings.Contains(line, "failed") {
				summary = strings.TrimSpace(line)
				passed = !strings.Contains(line, "failed") && strings.Contains(line, "passed")
				break
			}
		}
		if summary == "0/0 passed" {
			summary = strings.TrimSpace(outputStr)
		}
	}

	log.Printf("   %q test result: passed=%v, %s (%.0fms)", id, passed, summary, elapsed.Seconds()*1000)

	// Record in registry
	h.Reg.RecordTest(id, passed, summary, results)

	writeJSON(w, map[string]any{
		"plugin_id":  id,
		"passed":     passed,
		"summary":    summary,
		"results":    results,
		"elapsed_ms": elapsed.Milliseconds(),
	})
}

// ── Helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Ensure io is used (imported for potential future use)
var _ = io.Discard
