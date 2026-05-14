package handler

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

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

// Handler bundles all HTTP handlers with a registry and assignment store.
type Handler struct {
	Reg        *registry.Registry
	Assignments *AssignmentStore
	Proxies    map[string]*httputil.ReverseProxy // deployment → proxy
	proxyMu    sync.RWMutex
}

// New creates a Handler.
func New(reg *registry.Registry) *Handler {
	h := &Handler{
		Reg:        reg,
		Assignments: NewAssignmentStore(),
		Proxies:    make(map[string]*httputil.ReverseProxy),
	}
	// Load initial assignments from file
	h.Assignments.Load()
	// Initialize reverse proxies for known deployments
	h.refreshProxies()
	return h
}

// ── Assignment Store ─────────────────────────────────────────────────

// AssignmentStore manages vendor → deployment assignments.
// Backed by a YAML file (mounted from K8s ConfigMap).
type AssignmentStore struct {
	mu   sync.RWMutex
	data map[string]string // vendor_id → deployment_name
	path string
}

func NewAssignmentStore() *AssignmentStore {
	path := os.Getenv("ASSIGNMENTS_FILE")
	if path == "" {
		path = "/etc/plugin-hub/assignments.yaml"
	}
	return &AssignmentStore{
		data: make(map[string]string),
		path: path,
	}
}

func (a *AssignmentStore) Load() {
	a.mu.Lock()
	defer a.mu.Unlock()

	data, err := os.ReadFile(a.path)
	if err != nil {
		log.Printf("⚠️  No assignments file at %s — using empty", a.path)
		return
	}

	var raw struct {
		Assignments map[string]string `yaml:"assignments"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		log.Printf("⚠️  Failed to parse assignments: %v", err)
		return
	}

	a.data = raw.Assignments
	log.Printf("📋 Loaded %d vendor assignment(s)", len(a.data))
}

func (a *AssignmentStore) Save() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	raw := struct {
		Assignments map[string]string `yaml:"assignments"`
	}{Assignments: a.data}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(a.path, out, 0644)
}

func (a *AssignmentStore) Get(vendorID string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.data[vendorID]
}

func (a *AssignmentStore) Set(vendorID, deployment string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.data[vendorID] = deployment
}

func (a *AssignmentStore) All() map[string]string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make(map[string]string, len(a.data))
	for k, v := range a.data {
		out[k] = v
	}
	return out
}

func (a *AssignmentStore) Deployments() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	seen := make(map[string]bool)
	var result []string
	for _, d := range a.data {
		if !seen[d] {
			seen[d] = true
			result = append(result, d)
		}
	}
	return result
}

// ── Reverse Proxy ────────────────────────────────────────────────────

func (h *Handler) refreshProxies() {
	h.proxyMu.Lock()
	defer h.proxyMu.Unlock()

	// Known deployments from K8s services
	deployments := []string{
		"proxy-high",   // → ticket-proxy-high.plugin-hub.svc:8000
		"proxy-normal", // → ticket-proxy-normal.plugin-hub.svc:8000
	}

	for _, d := range deployments {
		svcHost := os.Getenv("PROXY_SVC_" + strings.ToUpper(strings.ReplaceAll(d, "-", "_")))
		if svcHost == "" {
			svcHost = "ticket-" + d + ".plugin-hub.svc.cluster.local:8000"
		}
		target, err := url.Parse("http://" + svcHost)
		if err != nil {
			log.Printf("⚠️  Invalid proxy target for %s: %v", d, err)
			continue
		}
		h.Proxies[d] = httputil.NewSingleHostReverseProxy(target)
		log.Printf("🔀 Proxy %s → %s", d, svcHost)
	}
}

// ProxyToVendor routes business API requests to the correct ticket-proxy pod.
func (h *Handler) ProxyToVendor(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")

	// Look up which deployment handles this vendor
	deployment := h.Assignments.Get(vendor)
	if deployment == "" {
		http.Error(w, `{"error":"vendor not assigned to any deployment"}`, 404)
		return
	}

	h.proxyMu.RLock()
	proxy, ok := h.Proxies[deployment]
	h.proxyMu.RUnlock()

	if !ok {
		http.Error(w, `{"error":"deployment not available"}`, 503)
		return
	}

	log.Printf("🔀 %s /api/v1/%s/* → %s", r.Method, vendor, deployment)
	proxy.ServeHTTP(w, r)
}

// ── Dashboard ────────────────────────────────────────────────────────

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]any{
		"Deployments": h.Assignments.Deployments(),
	}
	if err := dashTmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", 500)
	}
}

// ── Health ───────────────────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	stats := h.Reg.Stats()
	stats["deployments"] = h.Assignments.Deployments()
	writeJSON(w, stats)
}

// ── Plugin CRUD ──────────────────────────────────────────────────────

func (h *Handler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := h.Reg.ListAll()
	// Enrich with assignment info
	type PluginWithAssignment struct {
		*registry.Plugin
		Deployment string `json:"deployment,omitempty"`
	}
	result := make([]PluginWithAssignment, 0, len(plugins))
	for _, p := range plugins {
		result = append(result, PluginWithAssignment{
			Plugin:     p,
			Deployment: h.Assignments.Get(p.ID),
		})
	}
	writeJSON(w, result)
}

func (h *Handler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := h.Reg.Get(id)
	if !ok {
		http.Error(w, `{"error":"plugin not found"}`, 404)
		return
	}
	result := struct {
		*registry.Plugin
		Deployment string `json:"deployment,omitempty"`
	}{Plugin: p, Deployment: h.Assignments.Get(id)}
	writeJSON(w, result)
}

func (h *Handler) EnablePlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Reg.Enable(id); err != nil {
		writeJSONStatus(w, 403, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("✅ Plugin %q enabled", id)
	writeJSON(w, map[string]string{"status": "enabled", "plugin_id": id})
}

func (h *Handler) DisablePlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Reg.Disable(id); err != nil {
		writeJSONStatus(w, 403, map[string]string{"error": err.Error()})
		return
	}
	log.Printf("⏹  Plugin %q disabled", id)
	writeJSON(w, map[string]string{"status": "disabled", "plugin_id": id})
}

// ── Assignment API ───────────────────────────────────────────────────

// GetAssignments returns all vendor → deployment mappings.
func (h *Handler) GetAssignments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.Assignments.All())
}

// SetAssignment assigns a vendor to a deployment.
func (h *Handler) SetAssignment(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")

	var body struct {
		Deployment string `json:"deployment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONStatus(w, 400, map[string]string{"error": "invalid body"})
		return
	}

	if body.Deployment == "" {
		writeJSONStatus(w, 400, map[string]string{"error": "deployment is required"})
		return
	}

	// Verify vendor exists
	if _, ok := h.Reg.Get(vendor); !ok {
		writeJSONStatus(w, 404, map[string]string{"error": "vendor not found"})
		return
	}

	h.Assignments.Set(vendor, body.Deployment)
	if err := h.Assignments.Save(); err != nil {
		log.Printf("⚠️  Failed to save assignments: %v", err)
	}

	log.Printf("📋 Vendor %q → deployment %q", vendor, body.Deployment)
	writeJSON(w, map[string]string{
		"status":     "assigned",
		"vendor":     vendor,
		"deployment": body.Deployment,
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
