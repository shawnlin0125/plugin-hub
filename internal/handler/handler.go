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
	Reg    *registry.Registry
	proxy  *httputil.ReverseProxy // single proxy → all ticket-proxy pods
}

// New creates a Handler.
func New(reg *registry.Registry) *Handler {
	h := &Handler{
		Reg: reg,
	}
	h.refreshProxy()
	return h
}

// ── Reverse Proxy ────────────────────────────────────────────────────

func (h *Handler) refreshProxy() {
	svcHost := os.Getenv("PROXY_SVC")
	if svcHost == "" {
		svcHost = "ticket-proxy.plugin-hub.svc.cluster.local:8000"
	}
	target, err := url.Parse("http://" + svcHost)
	if err != nil {
		log.Printf("⚠️  Invalid proxy target: %v", err)
		return
	}
	h.proxy = httputil.NewSingleHostReverseProxy(target)
	log.Printf("🔀 Proxy → %s", svcHost)
}

// ProxyToVendor routes business API requests to ticket-proxy pods.
// Any enabled vendor is routed — all proxy pods load all enabled vendors.
func (h *Handler) ProxyToVendor(w http.ResponseWriter, r *http.Request) {
	vendor := r.PathValue("vendor")

	// Verify vendor exists AND is enabled
	p, ok := h.Reg.Get(vendor)
	if !ok {
		http.Error(w, `{"error":"vendor not found"}`, 404)
		return
	}
	if p.State != registry.StateEnabled {
		http.Error(w, `{"error":"vendor is not enabled"}`, 403)
		return
	}

	if h.proxy == nil {
		http.Error(w, `{"error":"proxy not available"}`, 503)
		return
	}

	log.Printf("🔀 %s /api/v1/%s/*", r.Method, vendor)
	h.proxy.ServeHTTP(w, r)
}

// ── Dashboard ────────────────────────────────────────────────────────

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashTmpl.Execute(w, nil); err != nil {
		http.Error(w, "template error", 500)
	}
}

// ── Health ───────────────────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, h.Reg.Stats())
}

// ── Plugin CRUD ──────────────────────────────────────────────────────

func (h *Handler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := h.Reg.ListAll()
	writeJSON(w, plugins)
}

func (h *Handler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := h.Reg.Get(id)
	if !ok {
		http.Error(w, `{"error":"plugin not found"}`, 404)
		return
	}
	writeJSON(w, p)
}

func (h *Handler) EnablePlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Reg.Enable(id); err != nil {
		writeJSONStatus(w, 403, map[string]string{"error": err.Error()})
		return
	}
	// Log the updated LOAD_PLUGINS value for ConfigMap sync
	enabled := h.Reg.EnabledVendors()
	log.Printf("✅ Plugin %q enabled — LOAD_PLUGINS=%s", id, enabledCSV(enabled))
	writeJSON(w, map[string]string{"status": "enabled", "plugin_id": id, "load_plugins": enabledCSV(enabled)})
}

func (h *Handler) DisablePlugin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Reg.Disable(id); err != nil {
		writeJSONStatus(w, 403, map[string]string{"error": err.Error()})
		return
	}
	enabled := h.Reg.EnabledVendors()
	log.Printf("⏹  Plugin %q disabled — LOAD_PLUGINS=%s", id, enabledCSV(enabled))
	writeJSON(w, map[string]string{"status": "disabled", "plugin_id": id, "load_plugins": enabledCSV(enabled)})
}

// ── Load Plugins Config (for proxy ConfigMap sync) ──────────────────

// GetLoadPlugins returns the comma-separated list of enabled vendors.
// Proxy pods use this value as LOAD_PLUGINS env var.
func (h *Handler) GetLoadPlugins(w http.ResponseWriter, r *http.Request) {
	plugins := h.Reg.EnabledVendors()
	writeJSON(w, map[string]string{"load_plugins": enabledCSV(plugins)})
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

func enabledCSV(vendors []string) string {
	s := ""
	for i, v := range vendors {
		if i > 0 {
			s += ","
		}
		s += v
	}
	return s
}
