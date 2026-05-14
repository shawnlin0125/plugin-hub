package handler

import (
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"

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
